# Employee Onboarding

Employee onboarding connects HR records, access control, and the tools a person needs to do their job.

## Typical flow

1. HRM creates or updates the employee record.
2. The employee position is confirmed.
3. Core Administration creates the user account.
4. Roles and permissions are assigned based on responsibility.
5. The employee signs in and checks access to required modules.
6. Managers review access after the first working period.

## What each module contributes

- HRM keeps the employee and position record.
- Core Administration controls user account, role, permission, and session access.
- OIDC may connect sign-in to the company identity provider.

## Business checks

- Is the employee record accurate?
- Does the position match the responsibilities?
- Are permissions limited to the role?
- Is sign-in working?
- Does the employee have access to the correct modules only?

## Best practices

- Assign access from roles instead of copying another user's permissions blindly.
- Review access again when the employee changes teams.
- Remove or suspend access quickly when someone leaves.
- Keep HRM and Core Administration in sync.
