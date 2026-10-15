# Security policy

## Supported versions

jamb is before 1.0. Fixes go into the latest release only.

| Version | Supported |
|---|---|
| 0.1.x | Yes |

## Reporting a vulnerability

Report it privately through GitHub:
[Report a vulnerability](https://github.com/tyler-builds/jamb/security/advisories/new). Please don't
open a public issue for it.

A useful report says which version and package are affected, how to reproduce the problem, and what an
attacker gains. jamb has one maintainer, so replies are best effort. Expect an answer within a week.
Once a fix is released, the advisory is published with credit to the reporter, unless they ask not to be named.

Vulnerabilities in a dependency (the Go standard library, `golang.org/x/...`, go-text, wazero) belong
with that project. If jamb has to change because of one, report that here too.
