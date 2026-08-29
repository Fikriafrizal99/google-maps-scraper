# Config-driven collector

This directory keeps reusable collector behavior outside the scraper core.

## Layout

- `presets/`: niche-specific search and post-processing rules.
- `areas/`: geographic search scopes.

The first preset is `kost`, and the first area is `jakarta`.

## Preset

A preset defines:

- search keywords
- required fields
- optional phone/website requirements
- minimum rating
- title include/exclude patterns
- deduplication key priority
- output fields

Example usage target:

```text
collect --preset kost --area jakarta
```

The CLI wiring will resolve the preset and area files, generate keyword + area search combinations, run the existing scraper, then apply filtering and deduplication.

## Area

An area defines a top-level search suffix and optional subareas. Keeping locations in config makes the same preset reusable across different cities.

For example, `kost` can later be combined with `bandung`, `surabaya`, or another area JSON without changing scraper code.

## Adding another niche

Create another file under `config/presets/`, for example:

```text
config/presets/wedding-organizer.json
config/presets/bengkel.json
config/presets/supplier-ikan.json
```

Do not add niche-specific conditions to the scraper core unless the rule cannot be represented by the shared config schema.
