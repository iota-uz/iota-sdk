# Local agent sign-in

Build both the CLI and server with `-tags dev`, set `APP_ENVIRONMENT=development`
and explicitly enable `AGENT_SIGNIN_ENABLED=true`. Register the controller returned
by `agentsignin.NewController(options)` in the host application. The host must
install its normal composition container and database context. The controller
installs CSRF protection and requires an exact same-origin POST.
Do not log request bodies on `/dev/agent-sign-in`.

The SDK CLI reads `AGENT_SIGNIN_ORIGIN` (an HTTP loopback origin) and
`AGENT_SIGNIN_TENANTID` (UUID). Run `agent sign-in --user <ID-or-email>`;
`--next /internal/path` selects the destination and `--output json` prints a
machine-readable URL. Consumers can mount `agentsignin.NewCommand(loadOptions)`
in their own Cobra CLI and supply their existing origin and tenant configuration.

Open the emitted URL in the agent's browser within one minute. Its fragment is
removed before a same-origin POST redeems the ticket. There is no account picker.
User lookup happens at redemption; unknown or ambiguous email addresses fail.
Use a numeric ID to disambiguate. No user or password is created or modified.

CLI and server must run as the same OS user with the same options. Tickets live
in a private user-cache directory scoped to origin and tenant; `Directory` /
`AGENT_SIGNIN_DIRECTORY` can override it. Consumed tickets are deleted. Unused
tickets expire and can be removed by deleting the corresponding cache directory.

Sessions use the normal SDK browser-session mechanism, expire after one hour,
and retain the user's real roles and permissions. Password, 2FA, onboarding,
blocked-account and host login-access checks are bypassed for these local
sessions. A distinct `agent-dev` audience is rejected by ordinary builds and
non-development environments. Other login methods keep their existing gates.

Production builds expose neither CLI command nor HTTP routes. Enabling the
controller in production fails startup. Remote clients, mismatched hosts and
cross-origin requests are refused. Local processes under the same OS account
are trusted; this tool is intended for a local development database.
