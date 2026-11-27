#!/usr/bin/env python3
"""Turn a pg_dump of a migrated database into a squashed sql-migrate baseline.

Usage:
  pg_dump --no-owner --inserts --exclude-table=public.gorp_migrations DB > dump.sql
  scripts/migrations-baseline.py dump.sql migrations/squashed/<batch> migrations/changes-0001-baseline.sql

The dump must come from a fresh database migrated with exactly the migrations
being squashed (no seed, no test data), with a pg_dump whose major version
matches the server. The second argument is the directory holding the squashed
migration files; each becomes a "-- +squashes" header line, by which the
migrator recognises a database built from the old history and refuses it
(see pkg/application/migrations_squash.go). TestSquashedBaselineMatchesHistory
compares the result against a replay of those files.

The output is written to a temporary file next to the target and moved into
place only when generation succeeds, so a refused dump leaves the existing
baseline untouched.

The dump is read through a small SQL lexer: pg_dump's table-of-contents
comments, psql meta-commands and session settings are recognised only between
statements, and rewrites apply only to SQL code, never to string literals or
dollar-quoted bodies. Every statement is emitted on its own; one that spans
lines is wrapped in StatementBegin/StatementEnd so sql-migrate never splits it
at a line that happens to end with a semicolon. The Down section drops every
object the baseline creates, kind by kind, with RESTRICT.
"""

import os
import re
import sys
import tempfile

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
# statements are dropped, never text inside a string or a function body.
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

# Object kinds the generated Down drops, in the order it drops them: views
# before the tables they read, tables before the sequences, functions, domains
# and types they use, aggregates before their functions. A dump with any other
# top-level object a rollback would have to remove is refused rather than
# producing a Down that leaves it behind.
DROPPABLE = ("MATERIALIZED VIEW", "VIEW", "TABLE", "SEQUENCE", "AGGREGATE", "FUNCTION", "PROCEDURE", "DOMAIN", "TYPE")
UNSUPPORTED = ("EXTENSION", "EVENT TRIGGER", "FOREIGN DATA WRAPPER", "SERVER", "PUBLICATION", "SUBSCRIPTION", "COLLATION", "OPERATOR")

TOC = re.compile(r"^-- Name: (.*); Type: (.*); Schema: (.*); Owner: .*$")
META = ("\\restrict ", "\\unrestrict ")

CODE, QUOTED, COMMENT = "code", "quoted", "comment"


def lex(sql):
    """Split SQL text into (kind, text) spans: code, quoted (string literals,
    quoted identifiers and dollar-quoted bodies) and comments."""
    spans, i, n, start = [], 0, len(sql), 0

    def flush(upto):
        if upto > start:
            spans.append((CODE, sql[start:upto]))

    while i < n:
        c = sql[i]
        if c in ("'", '"'):
            flush(i)
            j = i + 1
            while j < n:
                if sql[j] == c:
                    if j + 1 < n and sql[j + 1] == c:
                        j += 2
                        continue
                    break
                j += 1
            spans.append((QUOTED, sql[i:j + 1]))
            i = start = j + 1
            continue
        if c == "$":
            m = re.match(r"\$([A-Za-z_][A-Za-z0-9_]*)?\$", sql[i:])
            if m:
                tag = m.group(0)
                end = sql.find(tag, i + len(tag))
                if end < 0:
                    raise SystemExit("unterminated dollar quote at offset %d" % i)
                flush(i)
                spans.append((QUOTED, sql[i:end + len(tag)]))
                i = start = end + len(tag)
                continue
        if sql.startswith("--", i):
            flush(i)
            j = sql.find("\n", i)
            j = n if j < 0 else j
            spans.append((COMMENT, sql[i:j]))
            i = start = j
            continue
        if sql.startswith("/*", i):
            flush(i)
            j = sql.find("*/", i + 2)
            spans.append((COMMENT, sql[i:j + 2]))
            i = start = j + 2
            continue
        i += 1
    flush(n)
    return spans


def split_statements(sql):
    """Split SQL text at top-level semicolons into lists of spans."""
    statements, current = [], []
    for kind, text in lex(sql):
        if kind != CODE:
            current.append((kind, text))
            continue
        while ";" in text:
            head, text = text.split(";", 1)
            current.append((CODE, head + ";"))
            statements.append(current)
            current = []
        if text:
            current.append((CODE, text))
    if current:
        statements.append(current)
    return statements


def take_preamble(spans):
    """Remove what precedes the statement proper: pg_dump's TOC comments,
    psql meta-commands and blank lines. Returns (toc entries, remaining spans)."""
    toc = []
    spans = list(spans)
    while spans:
        kind, text = spans[0]
        if kind == COMMENT and text.startswith("--"):
            m = TOC.match(text)
            if m:
                toc.append(m.groups())
            spans.pop(0)
            continue
        if kind == CODE:
            lines = text.split("\n")
            while lines and (lines[0].strip() == "" or lines[0].startswith(META)):
                lines.pop(0)
            if not lines:
                spans.pop(0)
                continue
            spans[0] = (CODE, "\n".join(lines))
        break
    return toc, spans


# pg_dump prints "varchar_col IN ('a', 'b')" as an ANY over a varchar array cast
# to text[], and that text does not parse back into the same expression: dumping
# the restored database prints each element cast instead. Writing the IN list
# back keeps the baseline's dump identical to the history's. The pattern spans
# its own string literals, so it runs on everything except dollar-quoted bodies,
# whose contents are data.
VARCHAR_IN = re.compile(
    r"\(\(([a-z_][a-z0-9_]*)\)::text = ANY \(\(ARRAY\[((?:'(?:[^']|'')*'::character varying(?:, )?)+)\]\)::text\[\]\)\)"
)


def restore_varchar_in(spans):
    def repl(m):
        values = re.findall(r"'(?:[^']|'')*'(?=::character varying)", m.group(2))
        return "(%s IN (%s))" % (m.group(1), ", ".join(values))

    out, region = [], []
    for kind, text in spans:
        if kind == QUOTED and text.startswith("$"):
            out.append(VARCHAR_IN.sub(repl, "".join(region)))
            region = []
            out.append(text)
        else:
            region.append(text)
    out.append(VARCHAR_IN.sub(repl, "".join(region)))
    return "".join(out)


def check_sql_migrate_lines(spans, stmt):
    """sql-migrate drops every physical line that starts with "-- ", even inside
    StatementBegin/StatementEnd, and reads "-- +" lines as its own commands.
    Such a line inside a literal or body would silently change the data, so it
    is refused."""
    for kind, text in spans:
        if kind == QUOTED:
            for line in text.split("\n")[1:]:
                if line.startswith("-- "):
                    raise SystemExit(
                        "a quoted value in this statement has a line starting with '-- ', which sql-migrate "
                        "would drop:\n%s" % stmt[:300])


def quote_ident(name):
    return '"' + name.replace('"', '""') + '"'


def qualified(schema, name, kind):
    """The SQL name for a TOC entry. TOC names are raw, so every part is quoted;
    a function's TOC name carries its argument types, already in SQL form."""
    if kind in ("FUNCTION", "PROCEDURE", "AGGREGATE"):
        fname, paren, args = name.partition("(")
        return "%s.%s%s%s" % (quote_ident(schema), quote_ident(fname), paren, args)
    return "%s.%s" % (quote_ident(schema), quote_ident(name))


def ends_with_semicolon(line):
    """sql-migrate's own test for a line that ends a statement."""
    prev = ""
    for word in line.split():
        if word.startswith("--"):
            break
        prev = word
    return prev.endswith(";")


def generate(dump, squashed_dir, out):
    schemas = []
    drops = {kind: [] for kind in DROPPABLE}

    # A setval to 1 with is_called false is the state CREATE SEQUENCE already
    # leaves, but only for a sequence that starts at 1.
    starts_at_one = set(re.findall(r"CREATE SEQUENCE (\S+)\n(?:    .*\n)*?    START WITH 1\n", dump))

    statements = []
    for spans in split_statements(dump):
        toc, spans = take_preamble(spans)
        for name, kind, schema in toc:
            if kind == "SCHEMA":
                schemas.append(name)
            elif kind in drops:
                drops[kind].append(qualified(schema, name, kind))
            elif kind in UNSUPPORTED:
                raise SystemExit("the dump creates %s %s, which the generated Down cannot roll back" % (kind, name))
        if not spans:
            continue
        stmt = restore_varchar_in(spans).strip()
        if not stmt:
            continue
        if "\n" not in stmt and stmt.endswith(";") and any(stmt.startswith(p) for p in SKIP_PREFIXES):
            continue
        # Default privileges belong to whoever runs the migration, not to the
        # role that happened to build the dump.
        stmt = re.sub(r"^ALTER DEFAULT PRIVILEGES FOR ROLE \S+ ", "ALTER DEFAULT PRIVILEGES ", stmt)
        m = re.match(r"^SELECT pg_catalog\.setval\('([^']+)', 1, false\);$", stmt)
        if m and m.group(1) in starts_at_one:
            continue
        check_sql_migrate_lines(spans, stmt)
        lines = stmt.split("\n")
        if len(lines) == 1 and ends_with_semicolon(lines[0]):
            statements.append(stmt)
        else:
            statements.append("-- +migrate StatementBegin\n" + stmt + "\n-- +migrate StatementEnd")

    squashed = sorted(f for f in os.listdir(squashed_dir) if f.endswith(".sql"))
    if not squashed:
        raise SystemExit("no migrations in " + squashed_dir)

    out.write("-- Baseline: the schema and rows that migrations %s through %s produced.\n" % (squashed[0], squashed[-1]))
    out.write("-- Generated by scripts/migrations-baseline.py from pg_dump of a database migrated with\n")
    out.write("-- them; the originals are kept in %s for the equivalence test.\n" % squashed_dir)
    out.write("-- Do not edit: change the schema with a new migration.\n")
    out.write("--\n")
    out.write("-- A database built from the migrations below is refused, not upgraded: recreate it.\n")
    out.write("-- Only a full rollback (migrate down --all) runs this baseline's Down, which drops the schema.\n")
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
    out.write("-- Each kind of object goes in one statement with RESTRICT: dependencies among the\n")
    out.write("-- baseline's own objects are dropped together, while anything else that depends on them\n")
    out.write("-- (a table inheriting from one, a foreign key, a view, a table in an SDK schema) makes\n")
    out.write("-- the rollback fail and roll back instead of being dropped.\n")
    for kind in DROPPABLE:
        if drops[kind]:
            out.write("DROP %s IF EXISTS\n    %s\n    RESTRICT;\n" % (kind, ",\n    ".join(drops[kind])))
    for schema in schemas:
        if schema != "public":
            out.write("DROP SCHEMA IF EXISTS %s RESTRICT;\n" % quote_ident(schema))


def main():
    if len(sys.argv) != 4:
        raise SystemExit("usage: migrations-baseline.py DUMP SQUASHED_DIR OUTPUT")
    dump_path, squashed_dir, output = sys.argv[1:]
    with open(dump_path, encoding="utf-8") as f:
        dump = f.read()
    target_dir = os.path.dirname(os.path.abspath(output))
    fd, tmp = tempfile.mkstemp(prefix=".baseline-", suffix=".sql", dir=target_dir)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as out:
            generate(dump, squashed_dir, out)
        os.replace(tmp, output)
    except BaseException:
        os.unlink(tmp)
        raise


if __name__ == "__main__":
    main()
