Plan: Fix auto Readability fetch on article open

1. Add a failing unit test for auto-fetch eligibility that does not depend on the frontend link value.
2. Implement the minimal helper in `src/lib/readability.ts` and wire ReaderPage’s auto-fetch effect to it.
3. Run targeted test, then full `npm run test` and `npm run build`.
4. Update phase task/change docs and global CHANGE index.
