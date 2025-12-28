# Relish Text Representation (RTR)

A concise, human-writable text format for a single Relish Struct (and nested values) inspired by protobuf text format. RTR enables defining an object without writing Go code. A `.rlt` file encodes exactly one Relish Struct value.

This document specifies the syntax, type disambiguation, and precise TLV mapping, grounded in SPEC.md.

## Goals

- Human-friendly, diffable, and deterministic
- Lossless mapping to Relish TLV (no schema required)
- Type-precise where needed; sane defaults where unambiguous
- Strict validation (IDs, ordering, UTF-8, lengths) per SPEC.md

## File Structure

An RTR file contains an optional preamble of aliases, followed by a single Struct literal:

```
[Preamble]
Struct
```

### Comments and Whitespace

- Line comments: `# ...` or `// ...`
- Block comments: `/* ... */`
- Whitespace is insignificant except within strings

## Identifiers and Literals

- Ident: `[A-Za-z_][A-Za-z0-9_]*`
- Decimal int: `0|[1-9][0-9_]*` (underscores allowed as visual separators)
- Hex int: `0x[0-9A-Fa-f_]+`
- Float: `[0-9][0-9_]*\.[0-9_]+([eE][+-]?[0-9]+)?`
- String: double-quoted with C-style escapes (`\n`, `\t`, `\"`, `\\`, `\uXXXX`)

## Types

RTR exposes all Relish types:

- Fixed: `null`, `bool`, `u8`, `u16`, `u32`, `u64`, `u128`, `i8`, `i16`, `i32`, `i64`, `i128`, `f32`, `f64`, `timestamp`
- Varsize: `string`, `array<T>`, `map<K,V>`, `struct`, `enum`

Type names map 1:1 to SPEC.md Type IDs.

## Preamble (Optional Aliases)

Define friendly names for field IDs and, optionally, their expected value types, to improve readability and help inference.

Syntax:

```
let <Ident> = <FieldID> [ : <TypeHint> ] ;
```

- `<FieldID>`: decimal 0–127 (top bit must be clear)
- `<TypeHint>`: any of the Types above; container hints may be nested (e.g., `array<string>`, `map<string,u32>`, `struct`)

Examples:

```
let user_id = 1 : u64;
let name    = 2 : string;
let tags    = 3 : array<string>;
let props   = 4 : map<string,string>;
let created = 5 : timestamp;
let status  = 6 : enum;   # variant/value typed inline
```

## Values (Literals)

RTR uses typed literals and light annotations to eliminate ambiguity. When omitted, types are inferred where unambiguous; otherwise annotations are required.

### Scalars

- `null` → TypeNull
- `true` / `false` → TypeBool
- Integer with suffix selects exact integer type: `123u32`, `-5i16`, `0xffu8`, `1_000u64`
  - Without a suffix, integers are ambiguous; add a suffix or an explicit cast `(u32)123`
- Float: `3.14` (defaults to `f64`), add suffix for `f32` (e.g., `3.14f32`)
- String: `"hello"` (validated UTF-8) → TypeString
- Timestamp: `ts(1700000000)` or RFC3339 `ts("2023-10-01T12:00:00Z")` → TypeTimestamp

### Arrays

Two equivalent forms:

- Prefixed element type: `array<u32>[1u32, 2u32, 3u32]`
- Shorthand when all elements share a single, known type: `[1u32, 2u32]`

Rules:

- Element type must be determinable (either via prefix or consistent element literal types)
- Encodes as TypeArray with element type ID and element payloads per SPEC.md

### Maps

Typed form required unless all keys and all values have consistent, inferrable types:

- `map<string,u32>{ "a": 1u32, "b": 2u32 }`

Shorthand (when types are obvious and consistent):

- `{ "a": 1u32, "b": 2u32 }`

Rules:

- Duplicate keys are an error (per SPEC.md)
- Encodes as TypeMap with key/value type IDs and concatenated pair payloads

### Nested Structs

Write nested structs inline:

```
struct { <FieldEntry>* }
```

Where `FieldEntry` is described below. Nested structs are valid values for fields, arrays, or maps.

### Enums

`enum<VariantID>( Value )`

Example: `enum<1>("active")`, `enum<7>(123u32)`

Encodes as TypeEnum with the given variant ID and the contained value as a full TLV. The variant value must consume the exact enum content length (decoder validates this).

## Top-Level Struct

The file body is a single `struct { ... }` literal. Field order in the file is irrelevant; on encode, fields are sorted by Field ID to comply with SPEC.md.

### Field Entries

```
<FieldKey> ":" <Value> ";"?
```

Where `<FieldKey>` is either a numeric Field ID (0–127) or an alias defined in the preamble via `let`.

Examples:

```
struct {
  # using aliases
  user_id: 42u64;
  name: "Ada Lovelace";
  tags: ["math", "computing"];            # element type inferred = string
  props: map<string,string>{"role":"admin", "team":"analysis"};
  created: ts("2023-10-01T12:00:00Z");
  status: enum<1>("active");
}
```

Or without aliases:

```
struct {
  1: 42u64;
  2: "Ada Lovelace";
  3: array<string>["math", "computing"];
  4: map<string,string>{"role":"admin"};
  5: ts(1700000000);
  6: enum<1>("active");
}
```

### Omitting Optional Fields

Per SPEC.md, `Option<T>` fields that are `None` are omitted entirely. RTR treats the absence of a field as `None`. If you want to explicitly signal omission in text, use `none`:

```
status: none;   # encoder omits this field (no TLV emitted)
```

`none` is only meaningful for optional fields; otherwise it is an error.

## Type Annotations and Casts

Use explicit casts when a literal is ambiguous:

```
(u32) 123
(array<string>) ["a", "b"]
(map<string,u32>) {"a":1u32}
```

Cast syntax is `(<Type>) <Value>` and only affects the immediate literal.

## Shorthands

- Bytes as hex (array<u8>): `bytes(hex"dead_beef")` → `array<u8>` with content `DE AD BE EF`
- Element/entry separators: commas are optional for single-element arrays/maps; trailing commas allowed

## Canonicalization and Validation

- Struct: encoded with fields sorted by increasing Field ID; duplicate IDs are an error
- Enum: variant ID must have top bit clear; value TLV must consume full length
- Map: duplicate keys are an error; keys compared by value equality
- String: must be valid UTF-8
- Lengths: validate all varsize lengths; reject overflows or out-of-range values
- IDs: reject any Type/Field/Variant ID with top bit set

## TLV Mapping (Authoritative)

Given the parsed RTR value tree, write TLVs per SPEC.md:

- Scalar TLVs per their Type IDs
- Array: `[0x0F] [len] [elem_type_id] [elements...]`
  - Fixed elements: raw value bytes for each element (no type/len)
  - Varsize elements: `[len][content]` for each element (no type)
- Map: `[0x10] [len] [key_type_id] [value_type_id] [pairs...]` (same element rules as arrays)
- Struct: `[0x11] [len] [field_id_0][field_value_0 TLV] ...` (field IDs strictly increasing)
- Enum: `[0x12] [len] [variant_id] [variant_value TLV]`
- Timestamp: TypeTimestamp with little-endian u64 seconds since epoch

## Grammar (EBNF, abridged)

```
Document   = { Alias } Struct ;
Alias      = "let" Ident "=" FieldID [":" Type ] ";" ;
Struct     = "struct" "{" { FieldEntry } "}" ;
FieldEntry = FieldKey ":" (Value | "none") [";"] ;
FieldKey   = FieldID | Ident ;
FieldID    = DecInt ;
Value      = Cast | Scalar | Array | Map | Struct | Enum ;
Cast       = "(" Type ")" Value ;

Scalar     = Null | Bool | Int | Float | String | Timestamp ;
Null       = "null" ;
Bool       = "true" | "false" ;
Int        = (DecInt | HexInt) [IntSuffix] ;
IntSuffix  = "u8"|"u16"|"u32"|"u64"|"u128"|"i8"|"i16"|"i32"|"i64"|"i128" ;
Float      = FloatLit [ ("f32" | "f64") ] ;
String     = DQString ;
Timestamp  = "ts" "(" ( DecInt | DQString ) ")" ;

Array      = [ "array" "<" Type ">" ] "[" [ Value { "," Value } ] "]" ;
Map        = [ "map" "<" Type "," Type ">" ]
             "{" [ Value ":" Value { "," Value ":" Value } ] "}" ;
Enum       = "enum" "<" FieldID ">" "(" Value ")" ;

Type       = "null"|"bool"|"u8"|"u16"|"u32"|"u64"|"u128"|
             "i8"|"i16"|"i32"|"i64"|"i128"|"f32"|"f64"|
             "timestamp"|"string"|("array" "<" Type ">")|("map" "<" Type "," Type ">")|"struct"|"enum" ;
```

## Worked Examples

Minimal, numeric-only:

```
struct {
  0: null;
  1: true;
  2: 255u8;
  4: 1_000u32;
  14: "hi";
  15: array<u16>[1u16, 2u16, 3u16];
  16: map<string,u32>{"x":1u32, "y":2u32};
  17: struct { 0: "nested"; 1: 7u8; };
  18: enum<3>("variant-payload");
  19: ts("2024-01-01T00:00:00Z");
}
```

Readable with aliases:

```
let id      = 1 : u64;
let name    = 2 : string;
let tags    = 3 : array<string>;
let meta    = 4 : map<string,string>;
let created = 5 : timestamp;
let status  = 6 : enum;

struct {
  id: 42u64;
  name: "Relish";
  tags: ["fast", "compact"];
  meta: {"lang": "go", "spec": "v1"};
  created: ts(1700000000);
  status: enum<1>("ready");
}
```

## Notes and Guidance

- Without external schema, field names require `let` aliases; otherwise use numeric IDs
- Prefer explicit integer/float suffixes to avoid ambiguity
- For arrays/maps, include the type prefix unless every element unambiguously determines a single type
- Omit optional fields rather than encoding `null` when representing `None`
- Keep files UTF-8; escape non-printable characters in strings

## Future Work

- Optional: reference external schemas for field names/types
- A small CLI (`rltc`) to validate and encode `.rlt` → Relish TLV using this repository’s Encoder
- Editor support (syntax highlighting) and golden test vectors in `testdata/`

