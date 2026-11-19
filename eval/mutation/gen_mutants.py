#!/usr/bin/env python3
"""Generate the AtomiCos mutant set for cosmos/evm v0.7.4 (pre-registered, see README.md).

    python3 gen_mutants.py <clean cosmos/evm v0.7.4 tree> <out dir>

Writes <out>/mutants.tsv (one row per mutant) and <out>/patches/<id>.patch (git-apply-able).
Everything is mechanical: the operators, target files, contexts and the run order (fixed seed)
are defined in this file and must not be changed after results have been seen.
"""
import difflib
import os
import random
import re
import sys

SEED = 20261119

# ------------------------------------------------------------------ targets (README §2)
TARGETS = [
    "precompiles/common/precompile.go",
    "precompiles/common/balance_handler.go",
    "precompiles/common/utils.go",
    "x/vm/statedb/statedb.go",
    "x/vm/statedb/journal.go",
    "x/vm/statedb/state_object.go",
    "x/vm/keeper/statedb.go",
    "precompiles/staking/tx.go",
    "precompiles/distribution/tx.go",
    "precompiles/bank/bank.go",
    "precompiles/erc20/tx.go",
    "precompiles/erc20/approve.go",
    "precompiles/werc20/tx.go",
    "precompiles/gov/tx.go",
    "precompiles/slashing/tx.go",
    "precompiles/ics20/tx.go",
]

# calls that only log, trace or format: deleting them changes nothing the rules can see
SKIP_CALL = re.compile(
    r"(Logger\(|\.Debug\(|\.Info\(|\.Warn\(|span\.|StartSpan|EndSpan|^panic\(|^fmt\.|^_ = |"
    r"^defer |^go |EmitEvent\w*\(ctx, *[a-zA-Z]*Event|\.Emit\w*Event\()"
)
KEYWORDS = ("return", "if ", "for ", "switch", "case ", "default", "defer ", "go ", "var ",
            "func ", "else", "}", "{", "//", "const ", "type ", "import", "package", "break",
            "continue", "goto", "select", "fallthrough")

# B2 contexts: expression templates by receiver/parameter type (README §3.3)
CTX = {
    "NEST": {  # only after this transaction already made more than one precompile call
        "*StateDB": "{v}.precompileCallsCounter > 1",
        "*stateObject": "{v}.db.precompileCallsCounter > 1",
    },
    "LOCKED": {  # only for accounts that hold a locked (vesting) balance
        "*stateObject": "{v}.account.lockedBalance != nil && {v}.account.lockedBalance.Sign() > 0",
        "@locked": "locked != nil && locked.Sign() > 0",
    },
    "LOWGAS": {"@contract": "contract.Gas < 100000"},
    "CONTRACT": {"@contract": "contract.Caller() != evm.Origin"},
}
# functions where B2 variants are generated (core state-transition paths)
B2_FUNCS = {
    "runNativeAction", "AddPrecompileFn", "FlushToCacheCtx", "Commit", "commitWithCtx",
    "RevertToSnapshot", "Revert", "AddBalance", "SubBalance", "SetBalance",
    "SetBalanceWithLocked", "SetAccountBalance", "MultiStoreSnapshot", "RevertMultiStore",
}


def expected_rule(path, func, op):
    """Pre-registered expectation: which rule family should catch a mutant at this place."""
    if path.startswith("precompiles/") and path.endswith(("tx.go", "bank.go", "approve.go")):
        if "staking" in path or "distribution" in path:
            return "R3"
        return "-"  # no functional rule for these precompiles yet (expected miss)
    if func in ("Revert", "RevertToSnapshot", "RevertMultiStore", "MultiStoreSnapshot", "AddPrecompileFn"):
        return "R1"
    if func in ("runNativeAction", "HandleGasError"):
        return "R1|R2-W"
    if func in ("AddBalance", "SubBalance") and op == "GUARD":
        return "R2-A"
    if path.endswith(("balance_handler.go", "utils.go", "keeper/statedb.go", "state_object.go")):
        return "R2-W"
    if func in ("Commit", "commitWithCtx", "FlushToCacheCtx", "AddBalance", "SubBalance", "SetBalance"):
        return "R2-W"
    return "-"


# ------------------------------------------------------------------ source scanning
FUNC_RE = re.compile(r"^func (?:\((\w+) (\*?\w+)\) )?(\w+)\((.*)")


def parens_balance(s):
    depth = 0
    for ch in s:
        if ch in "([{":
            depth += 1
        elif ch in ")]}":
            depth -= 1
    return depth


def statement_end(lines, i):
    """Index of the last line of the statement starting at i (balanced brackets)."""
    depth = parens_balance(lines[i])
    j = i
    while depth > 0 and j + 1 < len(lines):
        j += 1
        depth += parens_balance(lines[j])
    return j


def block_end(lines, i):
    """Index of the closing brace of the block opened at the end of line i."""
    depth = 0
    for j in range(i, len(lines)):
        depth += lines[j].count("{") - lines[j].count("}")
        if depth == 0 and j > i:
            return j
    return None


def split_top(cond, sep=";"):
    depth = 0
    for k, ch in enumerate(cond):
        if ch in "([{":
            depth += 1
        elif ch in ")]}":
            depth -= 1
        elif ch == sep and depth == 0:
            return cond[:k], cond[k + 1:]
    return None, cond


class Fn:
    def __init__(self, name, recv_var, recv_type, params):
        self.name, self.recv_var, self.recv_type, self.params = name, recv_var, recv_type, params

    def ctx_expr(self, ctx):
        spec = CTX[ctx]
        for typ, tpl in spec.items():
            if typ.startswith("@"):
                if re.search(r"\b%s\b" % typ[1:], self.params):
                    return tpl
                continue
            if self.recv_type == typ:
                return tpl.format(v=self.recv_var)
            m = re.search(r"(\w+) \%s\b" % re.escape(typ), self.params)
            if m:
                return tpl.format(v=m.group(1))
        return None


def scan(path, lines):
    """Yield (op, fn, start, end, new_lines, note) for every B1 site in one file."""
    fn = None
    i = 0
    while i < len(lines):
        line = lines[i]
        m = FUNC_RE.match(line)
        if m:
            fn = Fn(m.group(3), m.group(1), m.group(2), m.group(4))
        elif line.startswith("}"):
            fn = None
        s = line.strip()
        ind = line[: len(line) - len(line.lstrip())]
        if fn is None or not s or s.startswith("//"):
            i += 1
            continue

        # DEL: a bare call statement (possibly spanning lines)
        if (not s.startswith(KEYWORDS) and "(" in s and ":=" not in s
                and not re.match(r"^[\w\.\[\]\*]+\s*[-+*/]?=[^=]", s) and not SKIP_CALL.search(s)
                and re.match(r"^[\w\.\*\(\)\[\]]+\(", s)):
            j = statement_end(lines, i)
            if lines[j].rstrip().endswith(")"):
                yield ("DEL", fn, i, j, [], "delete call: " + s[:70])

        # ERR: `if ... err != nil {` whose body is a single return -> drop the return
        if re.match(r"^(} else )?if .*err != nil \{$", s):
            j = i + 1
            if j < len(lines) and lines[j].strip().startswith("return"):
                k = statement_end(lines, j)
                if k + 1 < len(lines) and lines[k + 1].strip() == "}":
                    yield ("ERR", fn, j, k, [], "ignore error: " + s[:70])

        # GUARD: `if COND {` whose block leaves (return/panic/continue/break) -> `false && (COND)`
        mg = re.match(r"^(\s*)(} else )?if (.+) \{$", line)
        if mg and not re.search(r"(^|; )err != nil$", mg.group(3).strip()):
            end = block_end(lines, i)
            body = lines[i + 1:end] if end else []
            if end and len(body) <= 12 and any(re.match(r"\s*(return|panic\(|continue|break)", b) for b in body):
                init, cond = split_top(mg.group(3))
                newcond = ("%s; " % init.strip() if init else "") + "false && (%s)" % cond.strip()
                new = "%s%sif %s {" % (mg.group(1), mg.group(2) or "", newcond)
                yield ("GUARD", fn, i, i, [new], "disable guard: " + s[:70])

        # SWAP: Add <-> Sub (balance changes and arithmetic)
        if not s.startswith("func "):
            for a, b in ((".AddBalance(", ".SubBalance("), (".SubBalance(", ".AddBalance("),
                         (".AddOverflow(", ".SubOverflow("), (".Add(", ".Sub("), (".Sub(", ".Add(")):
                if a in line:
                    yield ("SWAP", fn, i, i, [line.replace(a, b, 1)], "%s -> %s: %s" % (a, b, s[:60]))
                    break
        i += 1


# ------------------------------------------------------------------ explicit mutants
# Group A: public fixes reverted, expressed on v0.7.4 code (README §3.1).
# Each entry: id, advisory/commit, file, list of (old text, new text) applied in order, expected.
A_MUTANTS = [
    ("A01", "GHSA-mjfq 0fff8c14: journal entry only after a successful precompile call",
     "precompiles/common/precompile.go",
     [("\tif err := stateDB.AddPrecompileFn(snapshot); err != nil {\n\t\treturn nil, err\n\t}\n", ""),
      ("\treturn bz, nil\n}\n\n// SetupABI",
       "\tif err := stateDB.AddPrecompileFn(snapshot); err != nil {\n\t\treturn nil, err\n\t}\n\n\treturn bz, nil\n}\n\n// SetupABI")],
     "R1"),
    ("A02", "GHSA-8pfh 79089fee (1/2): revert does not restore the events of the reverted call",
     "x/vm/statedb/journal.go",
     [("\ts.cacheCtx.EventManager().OverrideEvents(pc.prevEvents)\n", "")], "R1-E|R2-W"),
    ("A03", "GHSA-8pfh 79089fee (2/2): one BalanceHandler shared by nested calls",
     "precompiles/common/balance_handler.go",
     [("func (bhf BalanceHandlerFactory) NewBalanceHandler() *BalanceHandler {\n\treturn &BalanceHandler{\n\t\tbankKeeper:    bhf.bankKeeper,\n\t\tprevEventsLen: 0,\n\t}\n}",
       "var atomicosSharedHandler *BalanceHandler\n\nfunc (bhf BalanceHandlerFactory) NewBalanceHandler() *BalanceHandler {\n\tif atomicosSharedHandler == nil {\n\t\tatomicosSharedHandler = &BalanceHandler{bankKeeper: bhf.bankKeeper}\n\t}\n\treturn atomicosSharedHandler\n}")],
     "R2-W"),
    ("A04", "#1187 ae2db4f1: locked balance read at commit time instead of the load-time snapshot",
     "x/vm/keeper/statedb.go",
     [("\tlocked := account.LockedBalanceSnapshot()\n\tif locked == nil {", "\tlocked := account.LockedBalanceSnapshot()\n\tif true || locked == nil {")],
     "R2-W"),
    ("A05", "fb82a3f8 (fork fix): module / blocked account check only when the balance grows",
     "x/vm/keeper/statedb.go",
     [("\tisBlockedChange := k.bankWrapper.BlockedAddr(cosmosAddr) && amount.ToBig().Cmp(coin.Amount.BigInt()) != 0\n\tif isModule || isBlockedChange {",
       "\tisBlockedChange := k.bankWrapper.BlockedAddr(cosmosAddr) && amount.ToBig().Cmp(coin.Amount.BigInt()) > 0\n\t_ = isModule\n\tif isBlockedChange {")],
     "R2-W"),
    ("A06", "GHSA-7g4w 0182da19: no underflow check in stateObject.SubBalance",
     "x/vm/statedb/state_object.go",
     [("\tif balance.Lt(amount) {\n\t\tpanic(", "\tif false && balance.Lt(amount) {\n\t\tpanic(")], "R2-A"),
    ("A07", "GHSA-367m c3ae9067 (1/3): non-precompile Commit writes straight into the tx context",
     "x/vm/statedb/statedb.go",
     [("\tcacheCtx, writeCache := s.ctx.CacheContext()\n\tif err := s.commitWithCtx(cacheCtx); err != nil {\n\t\treturn err\n\t}\n\twriteCache()\n\treturn nil\n}",
       "\treturn s.commitWithCtx(s.ctx)\n}")],
     "R1-S"),
    ("A08", "GHSA-367m c3ae9067 (2/3): precompile Commit flushes the cache before the dirty set",
     "x/vm/statedb/statedb.go",
     [("\t\tif err := s.commitWithCtx(s.cacheCtx); err != nil {\n\t\t\treturn err\n\t\t}\n\t\ts.writeCache()\n\t\treturn nil\n\t}",
       "\t\ts.writeCache()\n\t\treturn s.commitWithCtx(s.ctx)\n\t}")],
     "R1-S|R2-W"),
    ("A09", "c3ae9067 (3/3): no overflow check in stateObject.AddBalance",
     "x/vm/statedb/state_object.go",
     [("\tif overflowed {\n\t\tpanic(", "\tif false && overflowed {\n\t\tpanic(")], "R2-A"),
    ("A10", "GHSA-wm4f 4f50aab2: precompile call counter not enforced (DoS class)",
     "x/vm/statedb/statedb.go",
     [("\tif s.precompileCallsCounter >= types.MaxPrecompileCalls {", "\tif false && s.precompileCallsCounter >= types.MaxPrecompileCalls {")],
     "-"),
    ("A11", "GHSA-c6xv 8214c816: self-destruct allowed for contracts with staking positions",
     "x/vm/keeper/statedb.go",
     [("\tif hasPositions {\n", "\tif false && hasPositions {\n")],
     "-"),
]

# B1-LOCK: confusing spendable / locked / total balance (README §3.2, explicit list)
LOCK_MUTANTS = [
    ("x/vm/keeper/statedb.go", "\t\tk.SpendableCoin(ctx, addr),\n\t\tk.lockedCoin(ctx, addr),",
     "\t\tk.GetBalance(ctx, addr),\n\t\tk.lockedCoin(ctx, addr),", "StateDB loads the total balance instead of the spendable one", "R2-W"),
    ("x/vm/keeper/statedb.go", "\t\tk.lockedCoin(ctx, addr),\n\t\tk.GetCodeHash",
     "\t\tnew(big.Int),\n\t\tk.GetCodeHash", "locked balance snapshot taken as zero", "R2-W"),
    ("x/vm/keeper/statedb.go", "\tnewBalance := new(big.Int).Add(amount.ToBig(), locked)",
     "\tnewBalance := new(big.Int).Set(amount.ToBig())", "commit writes spendable only (locked dropped)", "R2-W"),
]


# ------------------------------------------------------------------ main
def unified(path, old, new):
    return "".join(difflib.unified_diff(old.splitlines(True), new.splitlines(True),
                                        "a/" + path, "b/" + path, n=3))


def main():
    tree, out = sys.argv[1], sys.argv[2]
    os.makedirs(os.path.join(out, "patches"), exist_ok=True)
    rows = []
    seen = set()

    def add(group, op, ctx, path, line, func, src, new, note, exp):
        if new == src:
            return
        key = (path, new)
        if key in seen:  # identical mutated file = duplicate mutant
            return
        seen.add(key)
        rows.append([group, op, ctx, path, str(line), func, exp, note.replace("\t", " "), unified(path, src, new)])

    srcs = {}
    for path in TARGETS:
        with open(os.path.join(tree, path), encoding="utf-8") as f:
            srcs[path] = f.read()

    # A
    for mid, note, path, edits, exp in A_MUTANTS:
        src = srcs[path]
        new = src
        for a, b in edits:
            if a not in new:
                print("%s: anchor not found in %s" % (mid, path), file=sys.stderr)
                new = None
                break
            new = new.replace(a, b, 1)
        if new:
            add("A", "FIX-REVERT", "-", path, 0, "-", src, new, "%s %s" % (mid, note), exp)

    # B1-LOCK
    for path, a, b, note, exp in LOCK_MUTANTS:
        if a in srcs[path]:
            add("B1", "LOCK", "-", path, 0, "-", srcs[path], srcs[path].replace(a, b, 1), note, exp)
        else:
            print("LOCK anchor not found: %s" % note, file=sys.stderr)

    # B1 operators and B2 contexts
    for path in TARGETS:
        src = srcs[path]
        lines = src.split("\n")
        for op, fn, i, j, repl, note in scan(path, lines):
            new_lines = lines[:i] + repl + lines[j + 1:]
            add("B1", op, "-", path, i + 1, fn.name, src, "\n".join(new_lines), note, expected_rule(path, fn.name, op))
            if fn.name not in B2_FUNCS or op == "SWAP":
                continue
            for ctx in CTX:
                expr = fn.ctx_expr(ctx)
                if not expr:
                    continue
                ind = re.match(r"\s*", lines[i]).group(0)
                if op in ("DEL", "ERR"):
                    body = lines[i:j + 1]
                    wrapped = ["%sif !(%s) {" % (ind, expr)] + ["\t" + b for b in body] + ["%s}" % ind]
                    new_lines = lines[:i] + wrapped + lines[j + 1:]
                else:  # GUARD: disable only inside the context
                    new_lines = lines[:i] + [repl[0].replace("false && (", "!(%s) && (" % expr, 1)] + lines[j + 1:]
                add("B2", op, ctx, path, i + 1, fn.name, src, "\n".join(new_lines),
                    "%s only when %s" % (note, ctx), expected_rule(path, fn.name, op))

    # ids and the fixed run order
    for k, r in enumerate(rows):
        r.insert(0, "%s%03d" % ({"A": "A", "B1": "M", "B2": "C"}[r[0]], k + 1) if r[0] != "A" else r[7].split()[0])
    order = [r for r in rows if r[1] == "A"]
    rest = [r for r in rows if r[1] != "A"]
    random.Random(SEED).shuffle(rest)
    rows = order + rest

    with open(os.path.join(out, "mutants.tsv"), "w", encoding="utf-8", newline="\n") as f:
        f.write("order\tid\tgroup\top\tctx\tfile\tline\tfunc\texpected\tnote\n")
        for n, r in enumerate(rows, 1):
            f.write("\t".join([str(n)] + r[:9]) + "\n")
            with open(os.path.join(out, "patches", r[0] + ".patch"), "w", encoding="utf-8", newline="\n") as p:
                p.write(r[9])
    by = {}
    for r in rows:
        by[(r[1], r[2])] = by.get((r[1], r[2]), 0) + 1
    print("mutants: %d" % len(rows))
    for k in sorted(by):
        print("  %-3s %-10s %d" % (k[0], k[1], by[k]))


if __name__ == "__main__":
    main()
