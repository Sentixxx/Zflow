Plan: Fix Readability button disabled issue

1. Add a small unit test capturing desired behavior: allow Readability action when an article exists even if link is empty.
2. Implement minimal helper for Readability eligibility and wire it into ReaderPage/toolbar props.
3. Update toolbar copy to reflect link availability separately from action availability.
4. Run targeted unit test, then full `npm run test` and `npm run build`.
5. Update phase task/change docs and global CHANGE index.
