# Sente starter configuration

This is the application release's optional snapshot of [DouwJacobs/sente-config](https://github.com/DouwJacobs/sente-config). Fresh databases contain no categories, spending groups, merchants or classification rules. Importing this snapshot is an explicit offline option in Settings → Configuration; the primary starter action pulls the public repository.

Maintain generic definitions in `sente-config/sente.json`, then refresh this release snapshot deliberately. Keep personal EFT patterns, account names, logos and other personal configuration in your own repository or private file.

The shared version 2 format and schema are documented in [docs/RULESETS.md](../../../docs/RULESETS.md). The schema is an authoring aid; runtime validation additionally checks combined entry limits, references, authorization, duplicate identities and dependencies.
