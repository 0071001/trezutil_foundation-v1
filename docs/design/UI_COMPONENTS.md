# Trezutil UI Components

## Overview

Reusable component library for Trezutil surfaces.

All components use design tokens and support dark mode.

---

## App Shell

### Structure

```
AppShell
├── Navbar
│   ├── Logo
│   ├── Navigation
│   ├── Search
│   └── UserMenu
├── Sidebar (Seller Dashboard)
│   ├── Navigation
│   └── Usage Summary
└── Main Content
```

### Props

| Prop | Type | Required | Description |
|------|------|----------|-------------|
| `children` | ReactNode | Yes | Main content |
| `navItems` | NavItem[] | Yes | Navigation items |
| `user` | User | Yes | Current user |
| `theme` | 'light' \| 'dark' | No | Theme mode |

---

## Navigation

### Component

```tsx
<Navigation items={navItems} activePath="/search" />
```

### NavItem

| Property | Type | Description |
|----------|------|-------------|
| `label` | string | Display label |
| `path` | string | Route path |
| `icon` | ReactNode | Optional icon |
| `badge` | number | Optional badge count |

---

## Layout

### Page Layout

```tsx
<PageLayout 
  title="Search"
  description="Find products and services"
>
  {children}
</PageLayout>
```

### Props

| Prop | Type | Required |
|------|------|----------|
| `title` | string | Yes |
| `description` | string | No |
| `children` | ReactNode | Yes |
| `actions` | ReactNode | No |
| `maxWidth` | string | No |

---

## Typography

### Heading Component

```tsx
<Heading level={1}>Page Title</Heading>
<Heading level={2}>Section Title</Heading>
<Heading level={3}>Card Title</Heading>
```

### Text Component

```tsx
<Text size="body" weight="normal">Body text</Text>
<Text size="small" weight="normal">Small text</Text>
<Text size="caption" weight="medium">Caption text</Text>
```

---

## Buttons

### Variants

```tsx
<Button variant="primary">Action</Button>
<Button variant="secondary">Cancel</Button>
<Button variant="ghost">Link</Button>
<Button variant="danger">Delete</Button>
```

### Props

| Prop | Type | Required | Description |
|------|------|----------|-------------|
| `variant` | 'primary' \| 'secondary' \| 'ghost' \| 'danger' | Yes | Style variant |
| `size` | 'sm' \| 'md' \| 'lg' | No | Size |
| `disabled` | boolean | No | Disabled state |
| `loading` | boolean | No | Loading state |
| `children` | ReactNode | Yes | Button content |

---

## Inputs

### TextInput

```tsx
<TextInput
  label="Search"
  placeholder="What are you looking for?"
  value={query}
  onChange={setQuery}
/>
```

### SelectInput

```tsx
<SelectInput
  label="Category"
  options={categories}
  value={selected}
  onChange={setSelected}
/>
```

### Props

| Prop | Type | Required |
|------|------|----------|
| `label` | string | Yes |
| `placeholder` | string | No |
| `value` | string | Yes |
| `onChange` | function | Yes |
| `error` | string | No |
| `disabled` | boolean | No |

---

## Cards

### ListingCard

```tsx
<ListingCard
  listing={listing}
  onClick={() => navigate(`/listings/${listing.id}`)}
/>
```

### Props

| Prop | Type | Required | Description |
|------|------|----------|-------------|
| `listing` | Listing | Yes | Listing data |
| `onClick` | function | No | Click handler |
| `image` | string | No | Custom image URL |

### Card Component

```tsx
<Card title="Title" description="Description">
  {children}
</Card>
```

---

## Badges

### Status Badge

```tsx
<Badge status="published">Published</Badge>
<Badge status="draft">Draft</Badge>
<Badge status="paused">Paused</Badge>
```

### Props

| Prop | Type | Required | Description |
|------|------|----------|-------------|
| `status` | string | Yes | Status value |
| `size` | 'sm' \| 'md' | No | Badge size |

### Status Colors

| Status | Background | Text |
|--------|------------|------|
| published | mint-light | mint-dark |
| draft | ice | secondary |
| paused | coral-light | coral |
| active | mint-light | mint-dark |
| unavailable | coral-light | coral |

---

## Status Indicators

### Usage

```tsx
<StatusIndicator
  label="Availability"
  status="available"
/>
```

### Props

| Prop | Type | Required |
|------|------|----------|
| `label` | string | Yes |
| `status` | 'available' \| 'unavailable' | Yes |
| `showLabel` | boolean | No |

---

## Search

### SearchBar

```tsx
<SearchBar
  value={query}
  onChange={setQuery}
  placeholder="What are you looking for?"
  onSearch={handleSearch}
/>
```

### Props

| Prop | Type | Required |
|------|------|----------|
| `value` | string | Yes |
| `onChange` | function | Yes |
| `placeholder` | string | No |
| `onSearch` | function | No |

---

## Conversation UI

### Message Component

```tsx
<Message
  text="Hello!"
  sender="user"
  timestamp={Date.now()}
/>
```

### Props

| Prop | Type | Required | Description |
|------|------|----------|-------------|
| `text` | string | Yes | Message text |
| `sender` | 'user' \| 'bot' | Yes | Sender type |
| `timestamp` | number | No | Unix timestamp |
| `cards` | Card[] | No | Structured content |

---

## Modal/Sheet

### Modal

```tsx
<Modal
  isOpen={isOpen}
  onClose={closeModal}
  title="Confirm Order"
>
  {content}
</Modal>
```

### Props

| Prop | Type | Required |
|------|------|----------|
| `isOpen` | boolean | Yes |
| `onClose` | function | Yes |
| `title` | string | Yes |
| `size` | 'sm' \| 'md' \| 'lg' | No |

---

## Loading States

### Skeleton

```tsx
<Skeleton lines={3} />
<Skeleton width="100%" height={200} />
```

### LoadingSpinner

```tsx
<LoadingSpinner size="md" />
```

---

## Empty States

```tsx
<EmptyState
  icon={SearchIcon}
  title="No results found"
  description="Try adjusting your search terms"
  action={<Button>Clear filters</Button>}
/>
```

---

## Error States

```tsx
<ErrorState
  code="NOT_FOUND"
  message="Page not found"
  retry={handleRetry}
/>
```

### Error Mapping

| Backend Code | UI State |
|-------------|----------|
| `MISSING_USER_ID` | Unauthorized |
| `INVALID_REQUEST` | Validation error |
| `NOT_FOUND` | Not found |
| `FORBIDDEN` | Unauthorized |
| `SEARCH_FAILED` | Server error |
| `INSUFFICIENT_CREDITS` | Quota exceeded |
