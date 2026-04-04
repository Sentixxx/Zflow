# Frontend Review Fixes Plan (Critical + Important)

- Add/adjust tests first (time formatting, translate stream parse, hasMore limit+1) to satisfy TDD.
- Implement critical fixes: local time formatting, guarded JSON.parse in streaming, single ApiClient source.
- Implement important fixes: remove useFeeds double source, extract shared bootstrap/actions, extract settings state, memoize folder/filters, add virtualization, implement limit+1 hasMore.
- Run `npm run test --if-present` and `npm run build`.
- Update phase task/change docs and global CHANGE.
- Commit and push only relevant files.
