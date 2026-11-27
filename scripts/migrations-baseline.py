#!/usr/bin/env python3
"""Turn a pg_dump of a migrated database into a squashed sql-migrate baseline.

Usage:
  pg_dump --no-owner --inserts --exclude-table=public.gorp_migrations DB > dump.sql
  scripts/migrations-baseline.py dump.sql migrations/squashed/<batch> > migrations/changes-0001-baseline.sql

The dump must come from a fresh database migrated with exactly the migrations
being squashed (no seed, no test data), with a pg_dump whose major version
matches the server. The second argument is the directory holding the squashed
migration files; each becomes a "-- +squashes" header line, which is how the
migrator recognises a database that already has the old history (see
pkg/application/migrations_squash.go). TestSquashedBaselineMatchesHistory
compares the result against a replay of those files.

Every statement is emitted on its own; a statement that spans lines is wrapped
in StatementBegin/StatementEnd so sql-migrate never splits it at a line that
happens to end with a semicolon. The Down section drops every object the
baseline creates.
"""

import os
import re
import sys

ROLES = """-- Roles are cluster-wide, so pg_dump does not carry them.
-- +migrate StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bichat_agent_role') THEN
        CREATE ROLE bichat_agent_role;
    END IF;
EXCEPTION
    WHEN duplicate_object OR unique_violation THEN
        NULL;
END
$$;
-- +migrate StatementEnd
"""

# Session settings pg_dump puts at the top of the dump. Only whole top-level
# statements are dropped, never lines inside a string or a function body.
SKIP_PREFIXES = (
    "SET statement_timeout",
    "SET lock_timeout",
    "SET idle_in_transaction_session_timeout",
    "SET transaction_timeout",
    "SET client_encoding",
    "SET standard_conforming_strings",
    "SELECT pg_catalog.set_config('search_path'",
    "SET check_function_bodies",
    "SET xmloption",
    "SET client_min_messages",
    "SET row_security",
    "SET default_tablespace",
    "SET default_table_access_method",
)

# Object types the generated Down knows how to drop. A dump with any other
# top-level object that a rollback would have to remove (an extension, say) is
# refused rather than producing a Down that leaves it behind.
DROPPABLE = ("MATERIALIZED VIEW", "VIEW", "TABLE", "SEQUENCE", "FUNCTION", "PROCEDURE", "AGGREGATE", "DOMAIN", "TYPE")
UNSUPPORTED = ("EXTENSION", "EVENT TRIGGER", "FOREIGN DATA WRAPPER", "SERVER", "PUBLICATION", "SUBSCRIPTION", "COLLATION", "OPERATOR")

TOC = re.compile(r"^-- Name: (.*); Type: (.*); Schema: (.*); Owner: .*$")


def split_statements(sql):
    """Split SQL text at top-level semicolons, honouring quotes, dollar quotes and comments."""
    out, buf, i, n = [], [], 0, len(sql)
    while i < n:
        c = sql[i]
        if c == "'" or c == '"':
            j = i + 1
            while j < n:
                if sql[j] == c:
                    if j + 1 < n and sql[j + 1] == c:
                        j += 2
                        continue
                    break
                j += 1
            buf.append(sql[i:j + 1])
            i = j + 1
        elif c == "$":
            m = re.match(r"\$([A-Za-z_][A-Za-z0-9_]*)?\$", sql[i:])
            if m:
                tag = m.group(0)
                end = sql.find(tag, i + len(tag))
                if end < 0:
                    raise SystemExit("unterminated dollar quote at offset %d" % i)
                buf.append(sql[i:end + len(tag)])
                i = end + len(tag)
            else:
                buf.append(c)
                i += 1
        elif sql.startswith("--", i):
            j = sql.find("\n", i)
            j = n if j < 0 else j
            buf.append(sql[i:j])
            i = j
        elif sql.startswith("/*", i):
            j = sql.find("*/", i + 2)
            buf.append(sql[i:j + 2])
            i = j + 2
        elif c == ";":
            buf.append(";")
            out.append("".join(buf))
            buf = []
            i += 1
        else:
            buf.append(c)
            i += 1
    rest = "".join(buf).strip()
    if rest:
        out.append(rest)
    return out


def strip_leading_comments(stmt):
    """Drop the pg_dump TOC comment block in front of a statement, and blank lines."""
    lines = stmt.strip("\n").split("\n")
    while lines and (lines[0].strip() == "" or lines[0].startswith("--")):
        lines.pop(0)
    return "\n".join(lines).strip()


# pg_dump prints "varchar_col IN ('a', 'b')" as an ANY over a varchar array cast
# to text[], and that text does not parse back into the same expression: dumping
# the restored database prints each element cast instead. Writing the IN list
# back keeps the baseline's dump identical to the history's.
VARCHAR_IN = re.compile(
    r"\(\(([a-z_][a-z0-9_]*)\)::text = ANY \(\(ARRAY\[((?:'(?:[^']|'')*'::character varying(?:, )?)+)\]\)::text\[\]\)\)"
)


def restore_varchar_in(stmt):
    def repl(m):
        values = re.findall(r"'(?:[^']|'')*'(?=::character varying)", m.group(2))
        return "(%s IN (%s))" % (m.group(1), ", ".join(values))
    return VARCHAR_IN.sub(repl, stmt)


def ends_with_semicolon(line):
    """sql-migrate's own test for a line that ends a statement."""
    prev = ""
    for word in line.split():
        if word.startswith("--"):
            break
        prev = word
    return prev.endswith(";")


def main():
    dump_path, squashed_dir = sys.argv[1], sys.argv[2]
    with open(dump_path, encoding="utf-8") as f:
        dump = f.read()

    schemas = []
    drops = {kind: [] for kind in DROPPABLE}
    for line in dump.split("\n"):
        m = TOC.match(line)
        if not m:
            continue
        name, kind, schema = m.groups()
        if kind == "SCHEMA":
            schemas.append(name)
        elif kind in drops:
            drops[kind].append("%s.%s" % (schema, name))
        elif kind in UNSUPPORTED:
            raise SystemExit("the dump creates %s %s, which the generated Down cannot roll back" % (kind, name))

    # \restrict and \unrestrict are psql meta-commands on lines of their own,
    # outside any statement.
    body = [l for l in dump.split("\n") if not (l.startswith("\\restrict ") or l.startswith("\\unrestrict "))]

    # A setval to 1 with is_called false is the state CREATE SEQUENCE already
    # leaves, but only for a sequence that starts at 1.
    starts_at_one = set(re.findall(r"CREATE SEQUENCE (\S+)\n(?:    .*\n)*?    START WITH 1\n", "\n".join(body)))

    statements = []
    for stmt in split_statements("\n".join(body)):
        stmt = restore_varchar_in(strip_leading_comments(stmt))
        if not stmt:
            continue
        if any(stmt.startswith(p) for p in SKIP_PREFIXES) and stmt.endswith(";") and "\n" not in stmt:
            continue
        # Default privileges belong to whoever runs the migration, not to the
        # role that happened to build the dump.
        stmt = re.sub(r"^ALTER DEFAULT PRIVILEGES FOR ROLE \S+ ", "ALTER DEFAULT PRIVILEGES ", stmt)
        m = re.match(r"^SELECT pg_catalog\.setval\('([^']+)', 1, false\);$", stmt)
        if m and m.group(1) in starts_at_one:
            continue
        lines = stmt.split("\n")
        if len(lines) == 1 and ends_with_semicolon(lines[0]):
            statements.append(stmt)
        else:
            statements.append("-- +migrate StatementBegin\n" + stmt + "\n-- +migrate StatementEnd")

    squashed = sorted(f for f in os.listdir(squashed_dir) if f.endswith(".sql"))
    if not squashed:
        raise SystemExit("no migrations in " + squashed_dir)

    out = sys.stdout
    out.write("-- Baseline: the schema and rows that migrations %s through %s produced.\n" % (squashed[0], squashed[-1]))
    out.write("-- Generated by scripts/migrations-baseline.py from pg_dump of a database migrated with\n")
    out.write("-- them; the originals are kept in %s for the equivalence test.\n" % squashed_dir)
    out.write("-- Do not edit: change the schema with a new migration.\n")
    out.write("--\n")
    out.write("-- A database that applied every migration below is recorded as having this baseline\n")
    out.write("-- without running it. Rolling it back drops the whole schema.\n")
    for name in squashed:
        out.write("-- +squashes %s\n" % name)
    out.write("\n-- +migrate Up\n\n")
    out.write(ROLES + "\n")
    out.write("-- Function bodies may name tables created further down.\n")
    out.write("SET LOCAL check_function_bodies = false;\n\n")
    for stmt in statements:
        out.write(stmt + "\n\n")

    out.write("-- +migrate Down\n\n")
    out.write("-- Roles are cluster-wide and may be used by other databases; they are left in place.\n")
    out.write("-- Each object the baseline created is dropped by name (CASCADE removes what depends on\n")
    out.write("-- it, as the squashed downs did); the schemas go last with RESTRICT, so a table someone\n")
    out.write("-- else created in one of them stops the rollback instead of being dropped with it.\n")
    for kind in DROPPABLE:
        for name in drops[kind]:
            out.write("DROP %s IF EXISTS %s CASCADE;\n" % (kind, name))
    for schema in schemas:
        if schema != "public":
            out.write("DROP SCHEMA IF EXISTS %s RESTRICT;\n" % schema)


if __name__ == "__main__":
    main()
