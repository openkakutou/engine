---
status: done
---
# Run CI On Push And Pull Request

## Description
Run build, test and vet on every push and PR, not only on tags.

## Acceptance Criteria
- [x] A workflow runs build, test and vet on every push and pull request, not only on version tags
- [x] A failing test makes the workflow fail, visible on the commit and PR
- [x] The tag-triggered release workflow is unchanged and still publishes its assets

## Notes
Today only .github/workflows/release.yml exists, triggered by tags, so a regression is discovered at release time. Follow the pinned-by-SHA convention of web-ui-kit's workflows. Reuse the release job's steps (including the WASM build) rather than duplicating logic where a reusable workflow fits.

## Progress
- 2026-11-21: `.github/workflows/ci.yml` runs build, vet, gofmt, test, the WASM build and its smoke on every push to main and every pull request, actions pinned by SHA as in web-ui-kit. The release workflow keeps publishing its assets (it now also runs the smoke, item 071).
