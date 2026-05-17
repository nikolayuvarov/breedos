# BreedOS

**A decision engine for artificial selection and CRISPR-enabled breeding.**

BreedOS helps breeding teams compare selection strategies across multiple generations — predicting genetic gain, tracking drift and inbreeding, and quantifying the long-term cost of short-term decisions.

## Core idea

Breeding is not only a prediction problem. It is, more fundamentally, a multi-generation **control problem** over an evolving population.

Most breeding software optimizes the next generation's prediction. BreedOS optimizes the trajectory. Aggressive selection can mathematically destroy the future search space — diversity collapses, rare beneficial alleles fixate or vanish, inbreeding rises. By generation 15, the population can stop responding to selection.

BreedOS makes these dynamics visible and comparable across strategies.

## What it does

The MVP runs Monte Carlo simulations across eleven selection strategies:

- **Neutral drift baseline** — no intentional selection; isolates drift, fixation, and mutation effects.
- **Aggressive selection** — maximizes short-term trait gain; useful as a warning baseline.
- **Diversity-preserving selection** — keeps genetically unusual candidates in the parent pool.
- **Balanced strategy** — default trade-off between near-term gain and long-term optionality.
- **Balanced + CRISPR seed** — edit-aware integration on a balanced background (CRISPR-enabled runs).
- **Random parent baseline** — randomly selects parents; separates selection value from drift noise.
- **Phenotype truncation selection** — classic truncation on noisy phenotype.
- **Genomic selection mock** — placeholder for GBLUP/Bayesian/ML predictors.
- **OCS-like constrained selection** — gain pursued under a similarity/diversity penalty.
- **Cross planner** — balanced parent choice plus more distant mating pairs.
- **Edit-aware introgression planner** — seeds lower-risk candidate edits through diversity-aware propagation (CRISPR-enabled runs).

Each run computes per-generation genetic gain, allele-frequency drift, inbreeding coefficient, diversity loss, and rare-allele tracking. Results are aggregated across replicates with worker-pool concurrency, scored on a risk-adjusted decision rank, and presented as a Pareto frontier on genetic gain × combined risk.

Core mode runs a four-strategy subset for fast comparison; advanced mode runs the full eleven.

## CRISPR positioning

BreedOS is **not** a guide-RNA design tool. That space is covered by Benchling, Synthego, and others.

BreedOS is a **decision layer** for CRISPR-enabled breeding: which candidate edits are worth introducing, in which genetic background, and how to propagate them through a breeding population without bottlenecks. Edit-aware introgression is one of the implemented selection strategies.

## Status

Early prototype. Working simulation engine, interactive demo UI, programmatic JSON API. Active development. Feedback from breeders, computational biologists, and CRISPR researchers is genuinely welcome — open an issue.

## Quickstart

Requires Go 1.23 or later.

```
git clone https://github.com/NikolayUvarov/breedos.git
cd breedos/mvp
go run .
```

Then open `http://127.0.0.1:8080/`.

### Demo pages

- `/` — landing page
- `/demo` — interactive simulation UI
- `/customer-discovery` — discovery framework view

## Run as a systemd service

For server deployment on a systemd-based Linux host, the repository ships an installer (`install.sh`) that wraps the binary in a managed service:

```
git clone https://github.com/NikolayUvarov/breedos.git
cd breedos
(cd mvp && go build -o ../breedos .)
sudo ./install.sh install
```

The installer prompts for command-line arguments (default for breedos: `-listen 0.0.0.0:8080`), the user to run as, and the working directory. After install it starts the service and prints the management commands.

Manage and inspect:

```
sudo systemctl status   breedos
sudo systemctl restart  breedos
sudo journalctl -u breedos -f
```

To inspect the unit file and recent logs without root: `./install.sh info`. To remove: `sudo ./install.sh uninstall`. Full options: `./install.sh help`.

## API

- `GET  /health` — health check
- `POST /api/simulate` — synchronous full run
- `POST /api/simulate/start` — async start (returns job ID)
- `GET  /api/simulate/status?id=<id>` — async poll

Request and response shapes live in the handler types at the top of `mvp/main.go`.

## Project structure

```
breedos/
├── README.md         (this file)
├── CHANGELOG.md
├── LICENSE
├── .gitignore
├── install.sh        systemd service installer (Linux)
└── mvp/
    ├── go.mod
    ├── main.go         simulation engine + HTTP server
    ├── main_test.go    tests
    └── static/         frontend (vanilla JS, no framework)
        ├── index.html
        ├── demo.html
        ├── customer_discovery.html
        ├── style.css
        └── app.js
```

## Tests

```
cd mvp
go test ./... -v
```

## Background

Built by Nikolay Uvarov. The thesis behind BreedOS comes from earlier academic work on the mathematical modeling of gene drift under artificial selection.

## Contributing

Early prototype under active development. If you'd like to contribute:

1. Open an issue describing what you want to change or add.
2. For non-trivial work, wait for a response before opening a PR.
3. Run `go test ./...` and `go vet ./...` before submitting.

## License

MIT — see [LICENSE](LICENSE).
