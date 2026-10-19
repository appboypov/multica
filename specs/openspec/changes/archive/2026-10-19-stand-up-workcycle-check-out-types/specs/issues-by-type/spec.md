## ADDED Requirements

### Requirement: Issues filters by type
Issues SHALL offer a Type filter with the eleven types and No type, each with its icon and its count. While types are picked, Issues SHALL show only issues of those types and a Type chip that removes the filter. A saved view SHALL keep its picked types.

#### Scenario: Only stand-ups
- **GIVEN** the workspace holds the stand-up WORK-1851 and the bug WORK-1890
- **WHEN** Brian picks Stand-up in the Type filter on Issues
- **THEN** Issues shows WORK-1851 and not WORK-1890, with a Type chip that says Stand-up

#### Scenario: Issues without a type
- **WHEN** Brian picks No type in the Type filter
- **THEN** Issues shows only the issues without a type

### Requirement: Issues groups by type
Board, List and Table SHALL offer grouping by Type, one group per type in template order and No type last. Moving a card to another type's column on Board MUST NOT change its type.

#### Scenario: Board by type
- **WHEN** Brian groups Board by Type
- **THEN** Board shows a column per type that has issues, in template order with No type last

#### Scenario: A card dropped on another type
- **GIVEN** Board is grouped by Type
- **WHEN** Brian drags the bug WORK-1890 to the Story column
- **THEN** WORK-1890 stays a bug in the Bug column

### Requirement: Issues sorts by type and shows a Type column
Issues SHALL offer sorting by Type in template order with No type last, and Table SHALL offer a Type column that shows each issue's type icon and name.

#### Scenario: Sorted by type
- **WHEN** Brian sorts List by Type ascending
- **THEN** epics come first and issues without a type come last

#### Scenario: The Type column
- **WHEN** Brian adds the Type column to Table
- **THEN** each row shows its issue's type icon and name, and an issue without a type shows No type
