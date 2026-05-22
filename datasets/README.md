# breedos/datasets/

Curated public wheat genotype datasets for BreedOS. The folder layout:

- `README.md` — this file. Tracked.
- `.gitignore` — ignores all dataset files (`*.vcf`, `*.zip`, `*.xlsb`, etc.). Tracked.
- **Everything else is gitignored.** Real data is downloaded locally per the fetch commands below, then deployed to the server by `deploy_breedos.sh`.

The machine-readable manifest is at [`../mvp/datasets-manifest.json`](../mvp/datasets-manifest.json) (lives in `mvp/` so the Go binary can `//go:embed` it without dot-dot path issues). Update that file when adding new datasets.

Why this layout: large genotype datasets (gigabytes) would bloat the repo forever and slow every clone. The repo carries metadata only; the operator fetches the actual files on their workstation, and the deploy script moves them to the server (in full for small datasets; truncated to the first 100 MB for large ones — adjustable per-deploy).

## Fetch — small datasets (all three together; total ~60 MB)

```bash
cd breedos/datasets
curl -L "https://ndownloader.figshare.com/files/21665928" -o wheat_durum_259x7817.vcf
curl -L "https://datadryad.org/downloads/file_stream/2980504" -o wheat_dryad_159_55k.xlsb
# Pakistani 37K consolidated genotypes file (find file_stream id on the landing page;
# Dryad serves files as redirects to S3 — copy the link from the dataset page).
# Landing: https://datadryad.org/dataset/doi:10.5061/dryad.xwdbrv1kb
```

## Fetch — large datasets

The first two are direct downloads and can be fetched headless. The Watkins
portal at https://wwwg2b.com is manual — visit the portal and save the
per-chromosome VCFs into this folder.

```bash
cd breedos/datasets

# INRAE 1000 wheat exomes (2.25 GB ZIP, ~5 min on 10 MB/s):
curl -L "https://urgi.versailles.inrae.fr/download/iwgsc/IWGSC_RefSeq_Annotations/v1.0/iwgsc_refseqv1.0_1000_wheat_exomes.zip" \
    -o iwgsc_refseqv1.0_1000_wheat_exomes.zip

# Zenodo 2.8M SNPs lifted to RefSeq v2.1 (9.2 GB VCF, ~15 min on 10 MB/s):
curl -L "https://zenodo.org/records/7852690/files/SNPs_lifted_final2_sorted_v1.vcf?download=1" \
    -o wheat_zenodo_2.8m_lifted_v21.vcf
```

## Deploy

`./deploy_breedos.sh` (run from `breedos/`) iterates `MANIFEST.json` and:

- For datasets with `"deploy_strategy": "full"` (small), uploads the entire file via `scp` to `<server>/data/datasets/<filename>` — with size-skip so unchanged files are not re-uploaded.
- For datasets with `"deploy_strategy": "truncate_head_100mb"` (large), uploads only the first 100 MB (the manifest's `deploy_truncate_mb` field controls the cap). Use `BREEDOS_DEPLOY_FULL_LARGE=1` to upload the full file (e.g., after a server-disk upgrade).
- For datasets with `"deploy_strategy": "manual"` (Watkins), skips automatic upload — operator handles those by hand.

After deploy, the live BreedOS exposes a dataset status page at `https://breedos.org/datasets` that reads the manifest and the actual on-server file sizes and renders: name, original size, downloaded size on server, content description, source URL, license.

## Conversion to BreedOS founder-CSV

`MANIFEST.json` describes the raw archives. The simulator's dataset loader (`mvp/dataset.go`) expects a BreedOS founder-CSV (rows = accessions, cols = SNPs, values = 0/1/2). The existing fetchers convert the raw data:

- `tools/data/fetch_arabidopsis_1001.py` — generic VCF → founder-CSV
- `tools/data/fetch_wheat_t3.py` — generic VCF/CSV/XLSX/ZIP → founder-CSV

For example, to convert the durum VCF after fetching:

```bash
python3 tools/data/fetch_wheat_t3.py \
    --source breedos/datasets/wheat_durum_259x7817.vcf \
    --out breedos/mvp/data/wheat_t3.csv \
    --n 200 --m 5000 --maf 0.05
```
