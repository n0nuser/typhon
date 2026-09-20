# Architecture decision records

One file per decision that was not forced, where a competent person could
reasonably have chosen otherwise. Each states the alternative it rejected and
what would make it wrong, because a decision record whose only content is the
option taken is a description, not a record.

Status is `accepted` unless something has since undermined it.

| # | Decision | Status |
| --- | --- | --- |
| [0001](0001-reimplement-the-rules-and-use-the-official-package-as-an-oracle.md) | Reimplement the rules; use the official package as a test oracle | accepted |
| [0002](0002-row-bitset-board-representation.md) | Represent the board as one `uint64` per row | accepted |
| [0003](0003-simultaneous-move-nodes.md) | Advance a node only once every snake has committed | accepted |
| [0004](0004-two-budget-modes.md) | Give the search a deadline budget or a node budget | accepted |
| [0005](0005-paranoid-with-opponent-reduction.md) | Paranoid opponents, and model only the nearest few | accepted |
| [0006](0006-ring-buffer-bodies-and-counted-occupancy.md) | Ring-buffer bodies, counted occupancy, explicit undo | accepted |
| [0007](0007-in-process-tournament-harness.md) | Run tournament games in process, not over HTTP | accepted |
| [0008](0008-safety-is-never-delegated.md) | Compute the safe move first and never delegate safety | accepted |
| [0009](0009-four-rulesets-and-a-loud-warning.md) | Implement four rulesets; play the rest loudly as standard | accepted |
| [0010](0010-state-is-single-goroutine.md) | A game state belongs to one goroutine | accepted |
