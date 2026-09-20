# Trezutil — Motion Guidelines

## Overview

Motion at Trezutil communicates state, progress, and continuity — not decoration.

---

## Principles

1. **Purposeful** — Every animation serves a product function
2. **Subtle** — Small, quick, not distracting
3. **Responsive** — Immediate feedback (< 100ms for interactions)
4. **Accessible** — Respect reduced-motion preferences
5. **Consistent** — Same motion patterns across all surfaces

---

## Motion Types

### Search Expansion

```css
.search-bar {
  transition: width var(--duration-normal) var(--ease-out);
}
```

Trigger: Focus on search bar.
Behavior: Smoothly expands from 200px to 400px.

---

### Result Reveal

```css
.result-card {
  animation: fadeInUp var(--duration-normal) var(--ease-out) both;
}

@keyframes fadeInUp {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}
```

Trigger: Search results loaded.
Behavior: Cards fade in sequentially (staggered 50ms).

---

### Listing Transition

```css
.listing-detail {
  animation: slideIn var(--duration-normal) var(--ease-out);
}

@keyframes slideIn {
  from { opacity: 0; transform: translateX(16px); }
  to { opacity: 1; transform: translateX(0); }
}
```

Trigger: Navigate to listing detail.
Behavior: Page slides in from right.

---

### Confirmation State

```css
.confirm {
  animation: pulse var(--duration-normal) var(--ease-out);
}

@keyframes pulse {
  0% { transform: scale(1); }
  50% { transform: scale(1.02); }
  100% { transform: scale(1); }
}
```

Trigger: Order confirmed.
Behavior: Brief scale pulse on success indicator.

---

### Conversation Message

```css
.message {
  animation: messageIn var(--duration-normal) var(--ease-out);
}

@keyframes messageIn {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}
```

Trigger: New message arrives.
Behavior: Message slides up slightly while fading in.

---

### Status Changes

```css
.status-badge {
  transition: background-color var(--duration-fast) var(--ease-in-out),
              color var(--duration-fast) var(--ease-in-out);
}
```

Trigger: Status changes (published → paused).
Behavior: Smooth color transition.

---

## Duration Reference

| Motion | Duration | Usage |
|--------|----------|-------|
| Instant feedback | 0ms | Hover states |
| Fast | 150ms | Toggles, small transitions |
| Normal | 250ms | Page transitions, expand/collapse |
| Slow | 400ms | Complex layouts, modals |

---

## Easing Reference

| Easing | Usage |
|--------|-------|
| `cubic-bezier(0.16, 1, 0.3, 1)` | Page transitions, list items |
| `cubic-bezier(0.4, 0, 0.2, 1)` | State changes, expansions |
| `cubic-bezier(0.34, 1.56, 0.64, 1)` | Confirmation, playful feedback |

---

## Reduced Motion

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

All animations are disabled for users with reduced-motion preferences.
Content remains fully accessible without animation.

---

## Don'ts

- Don't use animation as the only indicator (always include visual/text cues)
- Don't use bouncing or jarring animations
- Don't animate background colors on large elements
- Don't auto-play decorative animations
- Don't exceed 400ms for any single animation
