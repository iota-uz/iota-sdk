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

### Feature Documentation

**Before (Plain text):**
```
API Rate Limits
- Free tier: 100 requests/hour
- Pro tier: 1,000 requests/hour
- Enterprise: 10,000 requests/hour
```

**After (Component):**
```mdx
<InfoCard title="API Rate Limits">
  <InfoCard.Section title="Tier Limits">
    <InfoCard.Limit label="Free Tier">100 requests/hour</InfoCard.Limit>
    <InfoCard.Limit label="Pro Tier">1,000 requests/hour</InfoCard.Limit>
    <InfoCard.Limit label="Enterprise">10,000 requests/hour</InfoCard.Limit>
  </InfoCard.Section>
</InfoCard>
```

### Usage Analytics

**Before (Text table):**
```
Plan Type    | Monthly Cost
Free         | $0
Starter      | $29
Professional | $99
Enterprise   | $499
```

**After (Component):**
```mdx
<BarChart
  title="Plan Pricing"
  data={[
    { label: "Free", value: 0 },
    { label: "Starter", value: 29 },
    { label: "Professional", value: 99 },
    { label: "Enterprise", value: 499 }
  ]}
  maxValue={500}
/>
```

### Pricing Formula Documentation

**Before (Plain text):**
```
Monthly Cost = Base Price + (Active Users × Price Per User)
Base Price: $99
Active Users: 25
Price Per User: $5
Total: $224
```

**After (Component):**
```mdx
<FormulaBox title="Monthly Cost Calculation">
  <FormulaBox.Equation>
    Monthly Cost = Base Price + (Active Users × Price Per User)
  </FormulaBox.Equation>

  <FormulaBox.Variables>
    <FormulaBox.Var name="Base Price" value="$99" />
    <FormulaBox.Var name="Active Users" value="25" />
    <FormulaBox.Var name="Price Per User" value="$5" />
  </FormulaBox.Variables>

  <FormulaBox.Result label="Total Monthly Cost">
    $224
  </FormulaBox.Result>
</FormulaBox>
```

### Requirements Checklist

**Before (Bullet points):**
```
Required:
- Valid API key
- Node.js 18+
- PostgreSQL 13+

Optional:
- Redis cache
- Monitoring tools
```

**After (Component):**
```mdx
<ChecklistCard title="System Requirements">
  <ChecklistCard.Required>
    Valid API key
    Node.js 18+
    PostgreSQL 13+
  </ChecklistCard.Required>

  <ChecklistCard.NotRequired>
    Redis cache
    Monitoring tools
  </ChecklistCard.NotRequired>
</ChecklistCard>
```

## Complex Examples

### Nested Information with Multiple Sections

```mdx
<InfoCard title="Service Plans" icon={<span>📋</span>}>
  <InfoCard.Section title="Starter Plan">
    Perfect for small teams getting started
  </InfoCard.Section>

  <InfoCard.Section title="Features" nested>
    <InfoCard.Limit label="Users">Up to 10</InfoCard.Limit>
    <InfoCard.Limit label="Storage">100 GB</InfoCard.Limit>
    <InfoCard.Limit label="Support">Email</InfoCard.Limit>
  </InfoCard.Section>

  <InfoCard.Section title="Add-ons Available" nested>
    <p>Additional storage, priority support, and custom integrations</p>
  </InfoCard.Section>
</InfoCard>
```

### Combined Charts and Information

```mdx
<InfoCard title="Usage Analytics">
  <InfoCard.Section title="API Requests by Plan">
    <BarChart
      title="Monthly API Usage"
      data={[
        { label: "Free", value: 500 },
        { label: "Starter", value: 5000 },
        { label: "Pro", value: 25000 },
        { label: "Enterprise", value: 100000 }
      ]}
      maxValue={120000}
    />
  </InfoCard.Section>
</InfoCard>
```

### Calculation with Results

```mdx
<InfoCard title="Cost Estimation">
  <InfoCard.Section title="Calculate Your Monthly Bill">
    <FormulaBox>
      <FormulaBox.Equation>
        Total = Base Price + (Users × Price Per User) + (Storage × Storage Rate)
      </FormulaBox.Equation>
      <FormulaBox.Variables>
        <FormulaBox.Var name="Base Price" value="$99" />
        <FormulaBox.Var name="Users" value="25" />
        <FormulaBox.Var name="Price Per User" value="$5" />
        <FormulaBox.Var name="Storage" value="500 GB" />
        <FormulaBox.Var name="Storage Rate" value="$0.10/GB" />
      </FormulaBox.Variables>
      <FormulaBox.Result>$224</FormulaBox.Result>
    </FormulaBox>
  </InfoCard.Section>
</InfoCard>
```

## Where to Use

### Documentation Pages

Components enhance:
- **API documentation** - Rate limits, quotas, authentication
- **Pricing pages** - Plan comparisons, cost calculators
- **Feature guides** - Requirements, limitations, capabilities
- **Architecture docs** - System diagrams, data flows

### Component Organization

Place component examples in:
```
docs/
├── components/
│   ├── EXAMPLES.mdx        # All component examples
│   ├── INTEGRATION.md      # Integration guide (this file)
│   └── README.md           # Component documentation
```

## Migration Tips

### From Plain Text

1. Identify structured data (limits, calculations, comparisons)
2. Choose appropriate component (InfoCard, BarChart, FormulaBox)
3. Map data to component props
4. Add visual hierarchy with sections

### From Tables

Tables work for simple data, but components better for:
- Highlighting important values
- Showing relationships
- Interactive comparisons
- Visual impact

## Accessibility

All components support:
- **Keyboard navigation** - Tab through interactive elements
- **Screen readers** - Proper ARIA labels and descriptions
- **Reduced motion** - Respects `prefers-reduced-motion`
- **High contrast** - WCAG 2.1 AA compliant

## Dark Mode Support

Components automatically adapt to:
- Light mode (default)
- Dark mode (user preference or manual toggle)
- System preference (`prefers-color-scheme`)

No additional configuration needed!

## Next Steps

1. Browse `EXAMPLES.mdx` for all component variations
2. Check `README.md` for detailed component API
3. Start using components in your `.mdx` files
4. Customize styling with Tailwind classes if needed

## Support

For component questions or issues:
- Review the examples in this directory
- Check component source code in `components/`
- Open an issue on GitHub
