# 5. Detect an AppContainer as the second Windows mechanism

Date: 2026-08-25

## Status

Accepted

## Context

ADR 0004 gave the probe one Windows signal, `restricted-token`, which reads
`IsTokenRestricted` and detects Codex CLI's Windows sandbox. That detector is
correct and stays as it is.

It does not see GitHub Copilot CLI's sandbox. Measured on a Windows 11 host,
a probe run demonstrably confined by Copilot emitted `sandbox_detection: []`
and produced results identical to the unconfined run beside it. The probe was
blind to the enforcement, so the pair proved nothing.

ADR numbering is shared with `controlplaneio/sandbox-probe-reports`, which
holds 0001, 0003 and 0004. This is 0005.

### What Copilot's sandbox actually is

Copilot CLI's sandbox is Microsoft Execution Containers (MXC), one abstraction
over three OS backends: Seatbelt on macOS, bubblewrap on Linux and
**ProcessContainer** on Windows. The first two are already mechanisms the probe
detects, which is why only the Windows backend was invisible.

The name "ProcessContainer" suggests a Windows Server Silo. It is not one.
MXC's Windows runner calls `Experimental_CreateProcessInSandbox` in
`processmodel.dll`, passing a `SandboxSpec` FlatBuffer. Its `BaseContainerRunner`
always sets that spec's `app_container` field to true. The isolation is an
**AppContainer**, with job-object UI limits and process mitigations layered on
top.

### Why one bit, and which one

The `SandboxSpec` carries several things the probe could read:

| spec field | observable as | usable? |
| :--- | :--- | :--- |
| `app_container` | `TokenIsAppContainer` | **yes — always true** |
| `ui_restrictions` | `JOB_OBJECT_UILIMIT_*` flags | no — caller's policy |
| `disallow_win32k_system_calls` | process mitigation policy | no — caller's policy |
| `least_privilege` | stripped privilege set | no — caller's policy, and UAC-shaped |
| integrity level | `TokenIntegrityLevel` | no — caller's policy |

Only `app_container` is an invariant. Every other field is policy a caller
chooses, so its absence would prove nothing and a detector built on it would
report a real sandbox as absent whenever the caller turned that field off. The
same "read exactly one attested bit" discipline ADR 0004 applied to
`restricted-token` applies here, for the same reason.

`least_privilege` deserves the explicit note: it produces the deny-only groups
and stripped privileges that ADR 0004 already rejected, because an ordinary
non-elevated administrator's UAC split token is identical on both. It was
rejected once and is rejected again here.

## Decision

Add `app-container` as a second Windows **mechanism**, read from
`TokenIsAppContainer` on a real `TOKEN_QUERY` handle. It reuses the existing
`token.go` / `token_windows.go` seam rather than adding files, because it is
the same kind of fact: what this process's access token says about its own
confinement.

Like `restricted-token`, it also returns `RuntimeUnknown` from
`GetContainerRuntime`'s generic-fallback tier. That is not optional: the site's
`sandboxOf()` reports `"none"` for a row whose only values are mechanisms, so a
correctly detected confined run would otherwise render as unsandboxed.

It needs its own line in that tier rather than extending the restricted-token
branch. An AppContainer carries no restricting SIDs, so `IsTokenRestricted` is
false inside one and the existing branch does not fire.

### Two values, not one

`restricted-token` and `app-container` stay separate rather than collapsing
into one `windows-sandbox`. The primitives are independent — MXC builds an
AppContainer without calling `CreateRestrictedToken`, and Codex builds a
restricted token without an AppContainer — so keeping them apart is exactly
what lets the published data tell the two vendors' Windows sandboxes apart.
Collapsing them would throw that away to save one string.

### A mechanism, never a badge

An AppContainer token cannot say who created it. Store apps, Chromium's
renderer processes and Defender Application Guard all produce one. Naming
Copilot from this bit would repeat the `srt (linux)` mistake, so it is emitted
alongside the badge and never folded into it.

## Consequences

The false-positive risk here is **lower** than the restricted-token detector's,
not higher. A restricted token is nearly the shape of an ordinary UAC split
token, which is why ADR 0004 had to argue carefully about what not to read. An
AppContainer has no ambient analogue: a normal console process on a normal
desktop is not in one, and only something that deliberately built an
AppContainer can put the probe inside it.

`TokenIsAppContainer` is `TOKEN_INFORMATION_CLASS` 29 and `x/sys/windows` does
not export it — its iota run stops one short, at `TokenLogonSid`. The constant
is therefore written out here, and a test pins the literal against
`windows.TokenLogonSid+1`, so the two independent sources have to agree. A
future `x/sys` that inserts a class into that run fails the test instead of the
probe silently querying a different token property and reading the result as a
boolean.

### What is verified, and what is not

Verified: cross-compiles and vets for `windows/amd64`; the portable table test
covers all four combinations of the two token bools, proving each mechanism is
a pure function of its own bit; `TestIsAppContainerFalseForAnOrdinaryProcess`
runs the real API on a Windows runner and asserts the negative.

**Not verified: the positive case against a live MXC ProcessContainer.** The
evidence that `app_container` is always set is MXC's own source, not a
measurement of a running sandbox. The Windows `copilot-sandbox` matrix row is
what will close that, and until it runs green the positive case rests on
reading the vendor's code rather than on observing the vendor's behaviour.
