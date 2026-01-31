# MDX Components

Custom components for the Granite documentation site built with Next.js, Nextra, and Tailwind CSS v4.

## Components

### InfoCard

Nested info cards for displaying structured information with optional sections and limit callouts.

**Usage:**

```mdx
<InfoCard title="Coverage Details" icon={<span>📋</span>}>
  <InfoCard.Section title="Section 1">
    <p>Content here</p>
  </InfoCard.Section>

  <InfoCard.Section title="Nested Section" nested>
    <p>Indented content</p>
  </InfoCard.Section>

  <InfoCard.Limit label="Maximum">
    50,000,000 UZS
  </InfoCard.Limit>
</InfoCard>
```

**Props:**
- `title` (string, required): Card title
- `icon` (ReactNode, optional): Icon displayed next to title
- `children` (ReactNode, required): Card content

**Sub-components:**
- `InfoCard.Section`: Nested section with optional indentation
  - `title` (string): Section title
  - `nested` (boolean, optional): Adds left border and indentation
- `InfoCard.Limit`: Highlighted limit callout
  - `label` (string, optional): Label above the limit value

---

### BarChart

Horizontal bar chart for displaying coefficients, percentages, and comparative data. Pure Tailwind implementation without external charting libraries.

**Usage:**

```mdx
<BarChart
  title="Discount Coefficients"
  data={[
    { label: "Age 18-25", value: 1.5 },
    { label: "Age 26-35", value: 1.0 },
    { label: "Age 36-50", value: 0.9 },
    { label: "Age 50+", value: 1.2 }
  ]}
  maxValue={2}
/>
```

**Props:**
- `title` (string, required): Chart title
- `data` (BarChartData[], required): Array of `{ label: string, value: number }`
- `maxValue` (number, optional): Manual max value for scaling (auto-calculated if omitted)

---

### FormulaBox

Formula display with equation, variables, and result section for calculations.

**Usage:**

```mdx
<FormulaBox title="Premium Calculation">
  <FormulaBox.Equation>
    Premium = Base × Age Coefficient × Driver Coefficient
  </FormulaBox.Equation>

  <FormulaBox.Variables>
    <FormulaBox.Var name="Base" value="500,000 UZS" />
    <FormulaBox.Var name="Age Coef" value="1.5" />
    <FormulaBox.Var name="Driver Coef" value="0.8" />
  </FormulaBox.Variables>

  <FormulaBox.Result label="Total Premium">
    600,000 UZS
  </FormulaBox.Result>
</FormulaBox>
```

**Props:**
- `title` (string, optional): Box title
- `children` (ReactNode, required): Sub-components

**Sub-components:**
- `FormulaBox.Equation`: Displays formula in monospace code block
- `FormulaBox.Variables`: Container for variable definitions (grid layout)
  - `FormulaBox.Var`: Individual variable
    - `name` (string, required): Variable name
    - `value` (ReactNode, required): Variable value
- `FormulaBox.Result`: Highlighted result section
  - `label` (string, optional): Result label (default: "Result")

---

### ChecklistCard

Two-column checklist for displaying required and not-required items with visual indicators.

**Usage:**

```mdx
<ChecklistCard title="Document Requirements">
  <ChecklistCard.Required>
    Passport copy
    Vehicle registration
    Driver's license
  </ChecklistCard.Required>

  <ChecklistCard.NotRequired>
    Medical certificate
    Employment letter
  </ChecklistCard.NotRequired>
</ChecklistCard>
```

**Props:**
- `title` (string, optional): Card title
- `children` (ReactNode, required): Sub-components

**Sub-components:**
- `ChecklistCard.Required`: Items with green checkmark icon
- `ChecklistCard.NotRequired`: Items with red X icon

Each item should be on a new line (wrapped in `<ChecklistCard.Required>` or `<ChecklistCard.NotRequired>`).

---

## Dark Mode

All components support dark mode using Tailwind's `dark:` prefix. The site uses class-based dark mode where `.dark` class is applied to the HTML element.

Colors used:
- Light backgrounds: `bg-white`, dark: `dark:bg-gray-950`
- Light text: `text-gray-900`, dark: `dark:text-gray-100`
- Borders: `border-gray-200`, dark: `dark:border-gray-700`
- Accents: Blue (`blue-500`), Green (`green-500`), Red (`red-500`)

---

## Styling

Components use Tailwind CSS v4 utilities for consistent styling:
- Rounded corners: `rounded-lg` or `rounded-xl`
- Shadows: `shadow-sm`
- Spacing: `p-4` to `p-6`
- Responsive: Tailwind responsive prefixes (`sm:`, `md:`, etc.)

---

## Integration

Components are auto-registered in `mdx-components.tsx` and available in all `.mdx` files without additional imports.
