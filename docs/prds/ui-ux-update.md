# 📄 Product Requirements Document (PRD)

**Product Name:** Companion Dex
**Platform:** Mobile-first (iOS/Android responsive)
**Purpose:** A real-time companion app that tracks player progress, provides strategic recommendations, and integrates seamlessly with gameplay without requiring deep navigation.

---

## 1. 🎯 Objectives

### Primary Goals

* Provide a **centralized progress log** for players
* Deliver **context-aware recommendations** via the Professor system
* Minimize friction with a **single-page, scroll-first experience**
* Enhance gameplay decisions without interrupting play

### Success Metrics

* Daily active usage during gameplay sessions
* % of players interacting with Professor recommendations
* Average session time (short, frequent engagement preferred)
* Deck modifications influenced by recommendations

---

## 2. 👤 Target Users

### Core Audience

* Players actively engaged in the game loop
* Casual to mid-core strategy players
* Mobile-first users needing quick insights

### User Needs

* Quickly check progress and discoveries
* Understand weaknesses in their current setup
* Receive actionable suggestions without deep analysis

---

## 3. 📱 Core Experience

### Design Philosophy

* **Single Surface UI**: One main scrollable screen
* **Contextual Expansion**: Tap to reveal more, no page switching
* **Guided Intelligence**: System surfaces insights automatically

---

## 4. 🧱 Feature Breakdown

### 4.1 Home Screen (Primary Interface)

#### Header

* Player progress (% completion)
* Current objective (dynamic)
* Quick actions:

  * Scan (log encounter)
  * Search

---

### 4.2 Professor Insight Panel

#### Description

A persistent, dynamic recommendation module at the top of the screen.

#### Features

* Displays 1–3 prioritized recommendations
* Context-aware (deck, encounters, recent battles)
* Expandable for deeper explanation

#### Interactions

* Tap → Expand full insight
* Swipe → Cycle suggestions
* “Why?” → Show reasoning

---

### 4.3 Context Panel (Dynamic Section)

Changes based on player activity.

#### Modes

* **Deck View**: Shows current team and synergy gaps
* **Battle Prep**: Suggests counters and advantages
* **Exploration**: Shows recent or nearby encounters

---

### 4.4 Collection Feed

#### Description

Scrollable list of discovered entries (core interaction area)

#### Card States

**Collapsed**

* Name, number
* Sprite/image
* Type indicator
* Seen/Caught status

**Expanded (tap)**

* Stats (visual bars)
* Evolution preview
* Synergy tags
* “Add to Deck” action

---

### 4.5 Scan Function

#### Purpose

Quickly log encounters or unlock entries

#### UX

* Accessible via bottom navigation
* Fast input flow (minimal taps)
* Immediate feedback + animation

---

### 4.6 Profile / Progress

#### Features

* Overall completion stats
* Region/category breakdowns
* Achievement tracking

---

## 5. 🧭 Navigation

### Bottom Navigation (3 Items Max)

* Home (default)
* Scan
* Profile

### Design Rule

No deep navigation trees. All detail is handled via:

* Expandable cards
* Slide-up panels
* Overlays

---

## 6. 🧠 Intelligence System (Professor Logic)

### Inputs

* Player deck composition
* Encounter history
* Battle outcomes
* Card synergies

### Outputs

* Strategic recommendations
* Highlighted entries in feed
* Suggested deck changes

### Behavior Rules

* Prioritize most impactful insight
* Avoid repeating identical suggestions
* Adapt based on player response

---

## 7. 🎨 UI/UX Design Guidelines

### Visual Style

* Clean, modern sci-fi interface
* Subtle “device” feel (not overly skeuomorphic)

### Color System

* Neutral/dark base
* Accent colors based on types or categories

### Typography

* Clear hierarchy
* Emphasis on readability in short sessions

---

## 8. 🔄 Interaction Patterns

* Tap → Expand card
* Swipe → Navigate suggestions
* Long press → Quick preview
* Subtle animations for:

  * Unlocks
  * Recommendations
  * State changes

---

## 9. ⚙️ Technical Considerations (High-Level)

* Offline-capable for quick access
* Sync with game state (manual or API)
* Lightweight performance (fast load, low latency)

---

## 10. 🚀 Future Enhancements

* AI-driven recommendation improvements
* Deck auto-builder
* Social sharing (progress, builds)
* Event-based insights

---

# 🎨 Visual Reference (Design Brief for Mockup / Image Generation)

Use this to create or generate the UI:

---

## Screen Layout Description

**Mobile screen, dark theme**

### Top Header

* Thin bar with:

  * “Dex Progress: 42%”
  * Small icon buttons (Search, Scan)

---

### Professor Panel (Card Style)

* Rounded rectangle
* Slight glow/accent border
* Avatar icon (professor-like figure)
* Text:

  * “Your team is weak against Fire types”
* Small “Why?” button
* Expand icon

---

### Context Panel

* Horizontal scroll or card
* Example:

  * “Your Team”
  * 3–5 small creature icons
  * Highlight one missing type slot

---

### Collection Feed (Main Area)

Stacked cards:

Each card:

* Left: creature image
* Right:

  * Name + number
  * Type color strip
* Background: dark with subtle elevation

Expanded state:

* Stats bars
* Evolution chain
* Action button: “Add to Deck”

---

### Bottom Navigation

* 3 icons:

  * Home (selected)
  * Scan (center, emphasized)
  * Profile

---

### Motion Notes

* Cards expand smoothly downward
* Professor panel softly pulses when updated

---