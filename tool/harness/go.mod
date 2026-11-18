module atomicos/harness-src

// These files are not built here. inject.sh copies them into a cosmos/evm worktree,
// where they compile against that version. The separate go.mod keeps them out of
// the atomicos/tool module.

go 1.23
