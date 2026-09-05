# Sanjana in JaDE

The offline companion ships with JaDE; no Codex installation, credentials, or network service is required. Click Sanjana at the bottom of the editor to visit her corner. Cards are explicitly labeled samples. Hide Sanjana from her card and restore her using Show Sanjana in the header.

Fill in `character.md` when you are ready. It is a writing template for the future live companion, not an active prompt yet. No personality details are inferred from the artwork. Live news retrieval and generated dialogue are not connected in this prototype.

`spritesheet.png` is the browser copy of the Sanjana artwork. The Go server embeds it; the frontend plays the idle and waving rows. Rebuild with `npm run build` and restart/rebuild JaDE after code changes. Replacement artwork uses an 8-column, 9-row grid with 192 × 208 pixel cells.

Hide and still-animation preferences are stored in browser local storage per origin. They survive reloads on the same address; an automatically selected new port has separate preferences. Use a stable loopback address with JaDE's `--address` option if you want preferences across restarts. System reduced-motion settings also stop animation, and hidden tabs suspend it.

Future live updates should add a backend retrieval service with linked sources and publication dates, a result cache, and an explicit refresh action. Keep any service credentials outside the app bundle. Use the character guidance to shape commentary while keeping source facts separate.
