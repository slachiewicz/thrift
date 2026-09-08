# Compatibility and schema evolution

> **Applies to Apache Thrift 0.24.0.**

Thrift's field-ID scheme exists so schemas can evolve without a flag day.
This chapter defines the compatibility vocabulary, classifies the changes
you might make, shows the safe patterns as IDL diffs, and demonstrates
`thrift --audit` — the 0.24.0 compiler's mechanical compatibility check,
used throughout the example suite.

## Definitions

Consider reader R (old or new code) reading data W (old or new):

- **Backward compatibility** — new reader can read old data
  (upgrade the *reader* first).
- **Forward compatibility** — old reader can read new data
  (upgrade the *writer* first).

Both directions together = safe rolling upgrade. Thrift gives you both
*for free* as long as field IDs keep their meaning: unknown fields are
skipped by ID, known fields are matched by ID. Everything else — types,
requiredness, defaults — is where you can break it.

## Safe vs unsafe changes

| Change | Wire risk | Source/API risk | Recommended approach | Test required |
|---|---|---|---|---|
| add `optional` field with new ID | none (old readers skip; new readers see absence) | none (new attribute) | just do it; document in changelog | round-trip old↔new |
| add default-requiredness field with new ID | none on wire | readers see zero-value when absent | same as optional, plus decide if zero value is meaningful | new-reader read of old data |
| add `required` field | **breaks old readers** | — | don't. Add `optional`, promote only if a flag day is truly acceptable | — |
| stop writing a field (keep ID reserved) | none (readers skip) | deprecation in source | reserve the ID in a comment; never reuse | old reader reads new data |
| remove field and reuse ID for new type | **corrupts old readers' interpretation** | silent misbehavior | forbidden — reserve IDs | — |
| change field type (same ID) | **breaks both directions** unless semantics-preserving | breaks source | new ID + new field instead; read old, write new | audit must fail here |
| `optional` → `required` | **breaks readers of old data** | breaks writers omitting it | keep `optional`; enforce in handlers | audit must fail here |
| `required` → `optional` | old readers still demand the field — **writers that stop sending break them** | usually fine | only with coordinated writer upgrade; prefer additive | audit flags; review |
| add enum member (new value) | fine for readers that tolerate unknowns | switch statements may miss it | assign fresh value; handle "unknown" in handlers | cross-version round-trip |
| renumber/change enum value of existing member | **breaks both directions** | breaks comparisons | forbidden | — |
| add service method | none (old servers reply "unknown method" only if called) | none | safe; new client + old server must tolerate `TApplicationException` unknown-method | call test old/new |
| rename method | **breaks both directions** (name is the wire key) | breaks source | add new method; deprecate old | — |
| change method signature (add param with new ID) | old servers ignore the new arg; old clients fine if arg optional | re-generate | additive params only | cross-version call |
| change a default value | changes behavior of omitting readers/writers | behavior change | treat as API change; coordinate | integration test |
| change namespace/package | none on wire | code layout breaks | plan as source migration | build test |
| wrap field in new struct / move to new ID | type change — **breaks** | breaks source | new ID, keep old as `optional` during migration | audit must fail here |

Two *semantics-preserving* type swaps are worth knowing, but verify them
against your own consumers before relying on them: `byte`/`i8` are the
same wire type by definition (pure rename), and widening `i32` → `i64`
changes the wire type tag, so **it is not free** — readers must handle
both tags during migration (treat it as add-new-field, not in-place).

## The safe evolution playbook (worked example)

The example suite evolves v1 → v2 exactly this way
([reference_data_v2.thrift](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/reference_data_v2.thrift)):

```thrift
struct QuoteSnapshot {
  1: required uuid instrumentId,
  2: required AsOf asOf,
  3: optional double bid,
  4: optional double ask,
  5: optional double last,
  6: optional i64 volume,
  7: optional common.MarketPhase phase,

  // NEW in v2: additive, optional, previously unused IDs.
  8: optional QuoteCurrency quoteCurrency,
  9: optional double changePercent
}
```

and one new method appended to the *same* service:

```thrift
  list<QuoteSnapshot> latestSnapshots(
    1: list<uuid> instrumentIds,
    2: i32 maxResults
  ) throws (1: InvalidQuery invalidQuery)
```

with retirement documented, never reused:

```thrift
  // 12: (reserved, formerly sedol, withdrawn before release)
```

Mechanical check with the compiler:

```sh
thrift --audit reference_data.thrift reference_data_v2.thrift
```

Verified behavior of `--audit` on 0.24.0 (used to validate this exact
pair): exit code 0 and no output when compatible; exit code 2 with
per-field failure messages when not, e.g. from deliberately broken
variants of the example:

```
Struct Field Type Changed for Id = 5 in QuoteSnapshot
Struct Field removed for Id = 7 in QuoteSnapshot
Struct Field Requiredness Changed for Id = 6 in QuoteSnapshot
```

**Note:** `--audit` resolves `include`s per file; run it from a directory
where both files' includes resolve (or pass `-Iold dir` / `-Inew dir`).
Audit compares same-named declarations — evolving in place (same service
name, same namespaces) is what makes the check meaningful, and it is why
the example keeps the v1 names.

## Enum evolution details

```thrift
enum AssetClass {
  EQUITY       = 0,
  FIXED_INCOME = 1,
  FUND         = 2,
  FX           = 3,
  COMMODITY    = 4
}
```

Safe: append `CURRENCY = 5`. Unsafe: repurposing `FX`'s value.
Unknown-value handling is a runtime property: strict runtimes raise on
read; lenient ones expose the raw integer. Handlers should include a
default/unknown branch — this is application convention, not compiler
behavior.

## Service and method evolution

- **Adding a method** is safe for existing callers. A *new client calling
  the new method against an *old* server* receives the standard
  "unknown method / wrong type" application exception — catch it
  explicitly where feature detection matters.
- **Method deprecation**: stop documenting, keep serving. Removal is a
  contract break; schedule it like one (deprecate → burn-in period →
  major revision).
- **Service `extends`** creates a superset interface — useful for
  variant servers, not a versioning scheme. Prefer same-name additive
  evolution, as above.

## Defaults and requiredness drift

The two quiet compatibility hazards:

1. **Changing a default** changes old clients' behavior after
   regeneration (receiver applies the new default). See
   [IDL design](practical-guide/idl-design.html#default-values-are-part-of-the-interface).
2. **Demoting `required`** looks like a relaxation but old readers still
   *demand* the field. Any writer that stops sending it breaks them.

Run `--audit` in CI on every contract change; it turns both hazards into
build failures. (It cannot judge *semantic* changes — renaming field 3
from `last` to `mid` while keeping `double` passes the audit but breaks
meaning. Compatibility of meaning is a human review responsibility.)

## Compatibility review checklist

For every contract change, before merge:

- [ ] `thrift --audit old new` clean (or breakage explicitly reviewed and accepted)
- [ ] no field ID changed, removed, or reused without a reservation comment
- [ ] no existing field changed type or requiredness
- [ ] new fields are `optional` (or justified default-requiredness)
- [ ] enum members only appended with fresh values
- [ ] no method renamed; parameters only added with fresh IDs
- [ ] defaults unchanged, or change explicitly reviewed
- [ ] cross-version round-trip test added (old reader ← new writer, new reader ← old writer)
- [ ] consumers notified; changelog updated

## Release-upgrade review (Thrift runtime upgrades)

When upgrading the Thrift *runtime/compiler* itself (e.g. to a future
0.25.x), in addition to the above:

- [ ] re-read CHANGES.md for the target release, especially security and limits sections
- [ ] regenerated code diff reviewed (generator output can change formatting or APIs)
- [ ] cross-language test matrix executed (the example suite's pairings are a template)
- [ ] behavior defaults re-validated: size limits, TLS/hostname verification, timeouts
- [ ] consumers on older runtimes re-tested against the new producers
