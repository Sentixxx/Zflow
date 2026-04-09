# UI/UX Optimization Plan — 2026-04-09

## Background
UI/UX Pro Max audit identified 35 issues across 6 categories.
Priority: fix critical layout/interaction issues first, then accessibility, then polish.

## Scope
- Fix mobile layout overlaps (bottom nav, floating actions)
- Fix dialog UX (scroll lock, ESC key)
- Add cursor-pointer to all clickable elements
- Add loading states to async buttons
- Improve touch targets on mobile
- Add skeleton loading states
- Replace hardcoded colors with CSS variables
- Improve accessibility (aria-labels, keyboard nav)

## Files to Modify
1. `src/pages/ReaderPage.tsx` — mobile nav padding, dialog fix, z-index
2. `src/components/article-detail/ArticleFloatingActions.tsx` — z-index + mobile offset
3. `src/pages/BriefsPage.tsx` — cursor-pointer, skeleton loading, hardcoded colors
4. `src/components/layout/TopBar.tsx` — hardcoded colors
5. `src/components/sidebar/SidebarTree.tsx` — hardcoded icon colors, aria
6. `src/components/settings/SubscriptionSettingsCard.tsx` — loading spinner
7. `src/components/settings/AISettingsCard.tsx` — loading spinner
8. `src/components/ui/ToolbarIconButton.tsx` — touch target size
9. `src/components/article-list/ArticleListToolbar.tsx` — touch target size
10. `src/components/article-list/ArticleList.tsx` — skeleton rows

## Validation
- `npm run build` must pass
- Manual check: mobile layout, dark/light mode, keyboard nav
