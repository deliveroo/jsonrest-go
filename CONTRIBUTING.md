# Git

This project uses
[`semantic-release`](https://github.com/semantic-release/semantic-release)
to derive and generate [a version number and changelog from the commit
history](https://github.com/deliveroo/ad-platform/releases). This
enables us to share user-facing changes with external stakeholders in
a structured way, e.g.,
https://deliveroo.slack.com/messages/ad-platform-releases.

Git commit messages will be linted according to
[https://www.conventionalcommits.org/en/v1.0.0/]. See
[.commitlintrc.json](./.commitlintrc.json) for specific configuration.

```
<type>(<scope?>): <subject>
```

Note that any message with a `BREAKING CHANGE` will always create a
new major version. Do this with care as a major version bump for Go
libraries comes with some non-trivial overhead.
