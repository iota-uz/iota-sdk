# Integration Guide

How to use custom MDX components in your documentation.

## Quick Integration

Components are automatically available in all `.mdx` files. No imports needed!

Simply open any `.mdx` file and start using the components:

```mdx
---
title: Your Page Title
---

# Your Page

<InfoCard title="Your Title">
  Content here
</InfoCard>
```

## Real-World Examples

### Insurance Coverage Documentation

**Before (ASCII art):**
```
┌─────────────────────────────────────┐
│ OSAGO Coverage                      │
├─────────────────────────────────────┤
│ • Third party liability: 2M UZS     │
│ • Property damage: 1M UZS           │
│ • Passenger: 500K UZS               │
└─────────────────────────────────────┘
```

**After (Component):**
```mdx
<InfoCard title="OSAGO Coverage">
  <InfoCard.Section title="Coverage Details">
    <InfoCard.Limit label="Third Party Liability">2,000,000 UZS</InfoCard.Limit>
    <InfoCard.Limit label="Property Damage">1,000,000 UZS</InfoCard.Limit>
    <InfoCard.Limit label="Passenger">500,000 UZS</InfoCard.Limit>
  </InfoCard.Section>
</InfoCard>
```

### Premium Calculation Documentation

**Before (Text table):**
```
Age Group    | Coefficient
18-25        | 1.8
26-35        | 1.0
36-50        | 0.9
50+          | 1.2
```

**After (Component):**
```mdx
<BarChart
  title="Age-Based Premium Coefficients"
  data={[
    { label: "18-25", value: 1.8 },
    { label: "26-35", value: 1.0 },
    { label: "36-50", value: 0.9 },
    { label: "50+", value: 1.2 }
  ]}
  maxValue={2}
/>
```

### Formula Documentation

**Before (Plain text):**
```
Annual Premium = Base Rate * Age Coef * Driver Coef * Vehicle Coef
Base Rate: 500,000 UZS
Age Coefficient (26-35): 1.0
Driver Coefficient (Good): 0.95
Vehicle Coefficient (Sedan): 1.0
Total: 475,000 UZS
```

**After (Component):**
```mdx
<FormulaBox title="Annual Premium Calculation">
  <FormulaBox.Equation>
    Premium = Base Rate × Age Coef × Driver Coef × Vehicle Coef
  </FormulaBox.Equation>

  <FormulaBox.Variables>
    <FormulaBox.Var name="Base Rate" value="500,000 UZS" />
    <FormulaBox.Var name="Age Coef (26-35)" value="1.0" />
    <FormulaBox.Var name="Driver Coef (Good)" value="0.95" />
    <FormulaBox.Var name="Vehicle Coef (Sedan)" value="1.0" />
  </FormulaBox.Variables>

  <FormulaBox.Result label="Total Annual Premium">
    475,000 UZS
  </FormulaBox.Result>
</FormulaBox>
```

### Requirements Checklist

**Before (Bullet points):**
```
Required:
- Valid ID
- Vehicle registration
- Technical inspection

Not required:
- Medical certificate
- Employment proof
```

**After (Component):**
```mdx
<ChecklistCard title="Required Documents">
  <ChecklistCard.Required>
    Valid ID
    Vehicle registration
    Technical inspection
  </ChecklistCard.Required>

  <ChecklistCard.NotRequired>
    Medical certificate
    Employment proof
  </ChecklistCard.NotRequired>
</ChecklistCard>
```

## Complex Examples

### Nested Information with Multiple Sections

```mdx
<InfoCard title="Insurance Products" icon={<span>📋</span>}>
  <InfoCard.Section title="OSAGO (Mandatory)">
    Third-party liability insurance
  </InfoCard.Section>

  <InfoCard.Section title="Coverage Tiers" nested>
    <InfoCard.Limit label="Basic">500,000 UZS</InfoCard.Limit>
    <InfoCard.Limit label="Standard">1,000,000 UZS</InfoCard.Limit>
    <InfoCard.Limit label="Premium">2,000,000 UZS</InfoCard.Limit>
  </InfoCard.Section>

  <InfoCard.Section title="Optional Add-ons" nested>
    <p>Passengers, glass, theft protection available</p>
  </InfoCard.Section>
</InfoCard>
```

### Combined Charts and Information

```mdx
<InfoCard title="Premium Factors">
  <InfoCard.Section title="Discounts by Driving Record">
    <BarChart
      title="Premium Multipliers"
      data={[
        { label: "Excellent (0 claims)", value: 0.8 },
        { label: "Good (1-2 claims)", value: 1.0 },
        { label: "Fair (3-5 claims)", value: 1.5 },
        { label: "Poor (6+ claims)", value: 2.0 }
      ]}
      maxValue={2.5}
    />
  </InfoCard.Section>
</InfoCard>
```

### Calculation with Results

```mdx
<InfoCard title="Premium Calculation Example">
  <InfoCard.Section title="Step-by-Step Calculation">
    <FormulaBox>
      <FormulaBox.Equation>
        Final Premium = Base × Age × Driver Record
      </FormulaBox.Equation>
      <FormulaBox.Variables>
        <FormulaBox.Var name="Base" value="500,000 UZS" />
        <FormulaBox.Var name="Age (26-35)" value="1.0" />
        <FormulaBox.Var name="Driver (Good)" value="0.95" />
      </FormulaBox.Variables>
      <FormulaBox.Result>475,000 UZS</FormulaBox.Result>
    </FormulaBox>
  </InfoCard.Section>
</InfoCard>
```

## Where to Use

### Documentation Pages

1. **Policy Details** - Use InfoCard for coverage information
2. **Premium Guides** - Use BarChart for comparisons, FormulaBox for calculations
3. **Requirements** - Use ChecklistCard for document lists
4. **Product Specs** - Use InfoCard with nested sections
5. **Calculations** - Use FormulaBox for step-by-step math

### Recommended Locations in Granite Docs

```
docs/app/
├── [locale]/
│   ├── insurance/
│   │   ├── osago.mdx          ← Use InfoCard, BarChart
│   │   ├── kasko.mdx          ← Use InfoCard, ChecklistCard
│   │   └── premiums.mdx       ← Use FormulaBox, BarChart
│   ├── products/
│   │   └── [product].mdx      ← Use InfoCard, BarChart
│   └── requirements/
│       └── documents.mdx      ← Use ChecklistCard, InfoCard
```

## Styling & Customization

### Using Custom Icons

```mdx
<InfoCard
  title="Coverage"
  icon={<span>🛡️</span>}
>
  Content
</InfoCard>
```

Available icon options:
- 📋 Documents
- 🛡️ Protection/Coverage
- ✓ Verified/Approved
- ⚠️ Important/Warning
- 💰 Money/Cost
- 📊 Statistics/Data

### Adjusting Bar Chart Scale

```mdx
<!-- Auto-scale (0 to max value) -->
<BarChart title="Chart" data={[...]} />

<!-- Manual scale (0 to 10) -->
<BarChart title="Chart" data={[...]} maxValue={10} />
```

### Long Variable Names

For FormulaBox, use abbrev. and explain:

```mdx
<FormulaBox.Variables>
  <FormulaBox.Var
    name="ACF (Age Coef)"
    value="1.5 (age 18-25)"
  />
  <FormulaBox.Var
    name="DCF (Driver Coef)"
    value="0.95 (good record)"
  />
</FormulaBox.Variables>
```

### Multiple Checklists

```mdx
<ChecklistCard title="Requirements">
  <ChecklistCard.Required>
    Documents needed
  </ChecklistCard.Required>
  <ChecklistCard.NotRequired>
    Not needed
  </ChecklistCard.NotRequired>
</ChecklistCard>

<ChecklistCard title="Optional Features">
  <ChecklistCard.Required>
    Available add-ons
  </ChecklistCard.Required>
</ChecklistCard>
```

## Migration Checklist

When updating existing documentation:

- [ ] Identify ASCII art/tables that can be replaced
- [ ] Choose appropriate component:
  - InfoCard for structured info with sections
  - BarChart for comparisons/statistics
  - FormulaBox for calculations
  - ChecklistCard for requirements
- [ ] Update `.mdx` file with component syntax
- [ ] Remove old ASCII art/tables
- [ ] Test in development (`npm run dev`)
- [ ] Check dark mode appearance
- [ ] Verify responsive layout on mobile
- [ ] Commit changes

## Component API Reference

For complete component API, see:
- `/docs/components/README.md` - Full API reference
- `/docs/components/EXAMPLES.mdx` - Live examples

## Troubleshooting

### Component not appearing?

1. Check file is `.mdx` (not `.md`)
2. Verify component name capitalization
3. Check syntax matches examples
4. Restart dev server: `npm run dev`

### Styling looks wrong?

1. Check dark mode by toggling theme
2. Verify Tailwind classes in component
3. Check responsive layout on mobile
4. Review `styles/globals.css` for custom styles

### Data not displaying?

1. Check data array format: `[{ label, value }, ...]`
2. Verify values are numbers (not strings)
3. Check labels are strings
4. Test with simple data first

## Performance

Components are optimized for documentation:
- Zero external dependencies (except React)
- Pure Tailwind CSS styling
- No JavaScript overhead
- Fast builds and rendering
- Excellent SEO-friendly markup

## Accessibility

All components include:
- Semantic HTML structure
- Proper color contrast (WCAG AA)
- Clear visual hierarchy
- Icon with text labels
- Responsive to user preferences (dark mode)

No additional ARIA labels needed for most use cases.
