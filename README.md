# Introduction

Generates a CSV showing number of days each release of Go is current for. A release can be beta, release candidate or
general availability (1.x release).

Results: https://docs.google.com/spreadsheets/d/1754T8Y3bpaBXvZzNFBfVpnAWfvOcDukqMTiUEYIl074/edit#gid=0

Inspired by iOS version history: http://www.thinkybits.com/blog/iOS-versions/

Actual implementation may be poor and makes some assumptions, such as:

- Release candidate ends beta periods.
- General availability of a new minor ends the release candidate period.
- General availability of a new minor ends the previous GA (e.g. 1.6 ends the 1.5 branch, ignoring future 1.5.4 releases
    which are usually select security patches).
- All tags are tagged with the go as the prefix.
- Probably other assumptions which makes this go specific.

# Usage

```
go install github.com/bradleyfalzon/go-release-cycle
cd /path/to/go/src
git tag --format '%(refname),%(authordate)' --sort=authordate | go-release-cycle -show-rc -show-beta | sort
git tag --format '%(refname),%(authordate)' --sort=authordate | go-release-cycle -show-ga | sort
```
