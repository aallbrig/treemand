---
name: treemand-code-complexity
description: Measure treemand's code complexity and churn hotspots and decide what to refactor. Use for refactoring decisions, maintainability questions, or complexity findings.
---

# Code complexity

Follow **treemand-measure** for running, recording and filing; this skill is the complexity playbook.

**Measure**: `task measure -- complexity` (gocognit over non-test Go code, except `cognitive_avg`, which still averages test functions too, as its history does; git churn over 90 days). Metrics: `cognitive_max`, `cognitive_avg`, `functions_over_15`, `functions_over_30`, `files_over_800_lines`, `go_loc`, and the depth ratchets (none may grow): `file_lines.<path>` for each file over 800 lines, `dupl_clones` (clone groups of 50+ tokens, mibk/dupl) and `nestif_over_3` (ifs of nesting complexity 4+). `details` holds the top functions, the nested ifs and the **hotspots**: churn × the file's worst complexity.

**Choose by hotspot, not by maximum.** A complex function nobody changes costs little; the top hotspot is where every change is slow and risky. Refactor the top hotspot first, and one function per change. Splitting a function into one helper per case lowers the score without making anything deeper: shrink the interface or move duplicated policy into one module instead.

**Refactor safely**: the behaviour must not move. For the parser (`discovery/parse*.go`), `task corpus:diff` must print 0 lost, 0 gained and `go test ./discovery` must pass, before and after. Extract along the rules already named in comments (each section rule, flag shape, command-list shape is a candidate function or file). Elsewhere, the package's tests and `task ab` (for discovery) guard behaviour. Use **codebase-design** for where the seams go.

**Prove it**: the probe shows **improved** on the metric the issue named, and no other ratchet regressed.
