# Tailwind CSS v4 Migration - Before & After Examples

**Concrete examples showing exactly how to migrate your specific files**

---

## File 1: modules/core/presentation/assets/css/main.css

### BEFORE (v3)

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@font-face {
  font-family: "Inter";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Inter.var.woff2) format('woff2-variations');
  font-weight: 1 1000;
}

@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Regular.woff2) format('woff2');
  font-weight: 400;
}

@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Medium.woff2) format('woff2');
  font-weight: 500;
}

@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Semibold.woff2) format('woff2');
  font-weight: 600;
}

@layer base {
  :root {
    /* Base */
    --white: 100% 0 0;
    --black-800: 26% 0 0;
    --black: 18.67% 0 0;
    --black-950: 16.84% 0 0;
    --transparent: 0 0 0 / 0;

    /* Primary */
    --primary-400: 62.51% 0.172 283.89;
    --primary-500: 58.73% 0.23 279.66;
    --primary-600: 50% 0.192 279.97;
    --primary-650: 52.52% 0.272 280.57;
    --primary-700: 45.57% 0.171 280.34;

    /* Gray - Tailwind CSS v4 OKLCH palette */
    --gray-50: 98.5% 0.002 247.839;
    --gray-100: 96.7% 0.003 264.542;
    --gray-200: 92.8% 0.006 264.531;
    --gray-300: 87.2% 0.01 258.338;
    --gray-400: 70.7% 0.022 261.325;
    --gray-500: 55.1% 0.027 264.364;
    --gray-600: 44.6% 0.03 256.802;
    --gray-700: 37.3% 0.034 259.733;
    --gray-800: 27.8% 0.033 256.848;
    --gray-900: 21% 0.034 264.665;
    --gray-950: 13% 0.028 261.692;

    /* Red */
    --red-500: 59.16% 0.218 0.58;
    --red-100: var(--red-500) / 10%;
    --red-200: var(--red-500) / 20%;
    --red-300: var(--red-500) / 30%;

    /* Green */
    --green-50: 96.3% 0.0173 168.27;
    --green-100: 92.4% 0.0346 168.27;
    --green-200: 84.9% 0.0692 168.27;
    --green-500: 78.02% 0.1534 168.27;
    --green-600: 64.33% 0.1128 171.09;

    /* Pink */
    --pink-500: 59.16% 0.218 0.58;
    --pink-600: 58.93% 0.2129 359.93;

    /* Yellow */
    --yellow-500: 80.13% 0.1458 73.41;

    /* Blue */
    --blue-500: 82.1% 0.099263 240.9782;
    --blue-600: 64.12% 0.0638 240.72;

    /* Purple */
    --purple-500: 52.06% 0.2042 305.37;

    /* Sizes */
    --size-00: 0.25rem;
    --size-0: 0.4rem;
    --size-1: 0.5rem;
    --size-2: 0.75rem;
    --size-3: 1rem;
    --size-4: 1.25rem;
    --size-5: 1.5rem;

    /* ... rest of semantic tokens ... */
  }

  html.dark {
    /* Dark mode overrides */
    --clr-surface-100: var(--black-950);
  }
}

@layer components {
  .btn {
    /* ... */
  }
}

/* ... rest of file ... */
```

### AFTER (v4)

```css
@import "tailwindcss";

/* Content sources for Tailwind to scan */
@source "../../../**/templates/**/*.templ";
@source "../../../../components/**/*.go";

/* Theme tokens that generate Tailwind utilities */
@theme {
  /* Font families */
  --font-family-sans: "Gilroy", "Inter", system-ui, sans-serif;
  
  /* Base colors */
  --color-white: oklch(100% 0 0);
  --color-black: oklch(18.67% 0 0);
  --color-black-800: oklch(26% 0 0);
  --color-black-950: oklch(16.84% 0 0);
  
  /* Primary/Brand palette */
  --color-brand-400: oklch(62.51% 0.172 283.89);
  --color-brand-500: oklch(58.73% 0.23 279.66);
  --color-brand-600: oklch(50% 0.192 279.97);
  --color-brand-650: oklch(52.52% 0.272 280.57);
  --color-brand-700: oklch(45.57% 0.171 280.34);
  
  /* Gray scale */
  --color-gray-50: oklch(98.5% 0.002 247.839);
  --color-gray-100: oklch(96.7% 0.003 264.542);
  --color-gray-200: oklch(92.8% 0.006 264.531);
  --color-gray-300: oklch(87.2% 0.01 258.338);
  --color-gray-400: oklch(70.7% 0.022 261.325);
  --color-gray-500: oklch(55.1% 0.027 264.364);
  --color-gray-600: oklch(44.6% 0.03 256.802);
  --color-gray-700: oklch(37.3% 0.034 259.733);
  --color-gray-800: oklch(27.8% 0.033 256.848);
  --color-gray-900: oklch(21% 0.034 264.665);
  --color-gray-950: oklch(13% 0.028 261.692);
  
  /* Red with opacity variants */
  --color-red-500: oklch(59.16% 0.218 0.58);
  --color-red-100: oklch(59.16% 0.218 0.58 / 10%);
  --color-red-200: oklch(59.16% 0.218 0.58 / 20%);
  --color-red-300: oklch(59.16% 0.218 0.58 / 30%);
  --color-red-600: oklch(59.16% 0.218 0.58);
  --color-red-700: oklch(59.16% 0.218 0.58);
  
  /* Green */
  --color-green-50: oklch(96.3% 0.0173 168.27);
  --color-green-100: oklch(92.4% 0.0346 168.27);
  --color-green-200: oklch(84.9% 0.0692 168.27);
  --color-green-500: oklch(78.02% 0.1534 168.27);
  --color-green-600: oklch(64.33% 0.1128 171.09);
  
  /* Pink */
  --color-pink-500: oklch(59.16% 0.218 0.58);
  --color-pink-600: oklch(58.93% 0.2129 359.93);
  
  /* Yellow */
  --color-yellow-500: oklch(80.13% 0.1458 73.41);
  
  /* Blue */
  --color-blue-500: oklch(82.1% 0.099263 240.9782);
  --color-blue-600: oklch(64.12% 0.0638 240.72);
  
  /* Purple */
  --color-purple-500: oklch(52.06% 0.2042 305.37);
  
  /* Success alias */
  --color-success: oklch(78.02% 0.1534 168.27);
}

/* Font face declarations - unchanged */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Inter.var.woff2) format('woff2-variations');
  font-weight: 1 1000;
}

@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Regular.woff2) format('woff2');
  font-weight: 400;
}

@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Medium.woff2) format('woff2');
  font-weight: 500;
}

@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Semibold.woff2) format('woff2');
  font-weight: 600;
}

/* Semantic design tokens - keep in :root */
@layer base {
  :root {
    /* Transparent helper */
    --transparent: oklch(0 0 0 / 0);
    
    /* Sizes */
    --size-00: 0.25rem;
    --size-0: 0.4rem;
    --size-1: 0.5rem;
    --size-2: 0.75rem;
    --size-3: 1rem;
    --size-4: 1.25rem;
    --size-5: 1.5rem;

    --size-content-1: 20ch;
    --size-content-2: 45ch;
    --size-content-3: 60ch;

    /* Border */
    --border-size-1: 1px;

    /* Default btn */
    --clr-btn-bg: var(--transparent);
    --clr-btn-bg-active: var(--color-gray-200);
    --clr-btn-bg-hover: var(--color-gray-100);
    --clr-btn-text: var(--color-black);
    --crl-btn-text-hover: var(--color-black);
    --clr-btn-border: var(--transparent);
    --clr-btn-border-hover: var(--transparent);

    /* Primary btn */
    --clr-primary-btn-bg: var(--color-brand-500);
    --clr-primary-btn-bg-active: var(--color-brand-700);
    --clr-primary-btn-bg-hover: var(--color-brand-600);
    --clr-primary-btn-text: var(--color-white);
    --clr-primary-btn-text-hover: var(--color-white);
    --clr-primary-btn-border: var(--color-brand-600);
    --clr-primary-btn-border-hover: var(--color-brand-600);

    /* Secondary btn */
    --clr-secondary-btn-bg: var(--color-white);
    --clr-secondary-btn-bg-active: var(--color-gray-300);
    --clr-secondary-btn-bg-hover: var(--color-gray-200);
    --clr-secondary-btn-text: var(--color-black);
    --clr-secondary-btn-text-hover: var(--color-black);
    --clr-secondary-btn-border: var(--color-gray-300);
    --clr-secondary-btn-border-hover: var(--color-gray-300);

    /* Danger btn */
    --clr-danger-btn-bg: var(--color-red-100);
    --clr-danger-btn-bg-active: var(--color-red-300);
    --clr-danger-btn-bg-hover: var(--color-red-200);
    --clr-danger-btn-text: var(--color-red-500);
    --clr-danger-btn-text-hover: var(--color-red-500);
    --clr-danger-btn-border: var(--clr-danger-btn-bg);
    --clr-danger-btn-border-hover: var(--clr-danger-btn-bg);

    /* Primary outline btn */
    --clr-primary-outline-btn-text: var(--color-brand-500);
    --clr-primary-outline-btn-text-hover: var(--color-brand-500);
    --clr-primary-outline-btn-border: var(--color-brand-500);
    --clr-primary-outline-btn-border-hover: var(--color-brand-500);
    --clr-primary-outline-btn-bg-hover: oklch(58.73% 0.23 279.66 / 5%);

    /* Sidebar btn */
    --clr-sidebar-btn-bg: var(--transparent);
    --clr-sidebar-btn-bg-active: var(--color-brand-500);
    --clr-sidebar-btn-bg-hover: var(--color-brand-500);
    --clr-sidebar-btn-text: var(--color-gray-400);
    --clr-sidebar-btn-text-hover: var(--color-white);
    --clr-sidebar-btn-border: var(--transparent);
    --clr-sidebar-btn-border-hover: var(--color-brand-500);

    /* Form control */
    --clr-form-control-bg: var(--transparent);
    --clr-form-control-bg-active: var(--transparent);
    --clr-form-control-placeholder: var(--color-gray-400);
    --clr-form-control-border: var(--color-gray-300);
    --clr-form-control-border-hover: var(--color-brand-500);
    --clr-form-control-text: var(--color-black);
    --clr-form-control-ring: oklch(52.52% 0.272 280.57 / 20%);
    --clr-form-control-label: var(--color-gray-400);
    --form-control-border-size: var(--border-size-1);
    --form-control-size-x: calc(var(--size-2) - var(--form-control-border-size) * 2);
    --form-control-size-y: calc(var(--size-2) - var(--form-control-border-size) * 2);

    /* Surface */
    --clr-surface-50: var(--color-gray-50);
    --clr-surface-100: var(--color-gray-100);
    --clr-surface-200: var(--color-black);
    --clr-surface-300: var(--color-white);
    --clr-surface-400: var(--color-gray-200);
    --clr-surface-500: var(--color-gray-100);
    --clr-surface-600: var(--color-white);

    /* Text */
    --clr-text-100: var(--color-black);
    --clr-text-200: var(--color-gray-600);
    --clr-text-300: var(--color-gray-400);
    --clr-text-green: var(--color-green-500);
    --clr-text-pink: var(--color-pink-500);
    --clr-text-yellow: var(--color-yellow-500);
    --clr-text-blue: var(--color-blue-500);
    --clr-text-purple: var(--color-purple-500);

    /* Avatar */
    --clr-avatar-bg: oklch(58.73% 0.23 279.66 / 15%);
    --clr-avatar-text: var(--color-brand-500);

    /* Border */
    --clr-border-primary: var(--color-gray-300);
    --clr-border-secondary: var(--color-gray-100);
    --clr-border-green: var(--color-green-500);
    --clr-border-pink: var(--color-pink-500);
    --clr-border-yellow: var(--color-yellow-500);
    --clr-border-blue: var(--color-blue-500);
    --clr-border-purple: var(--color-purple-500);

    /* Table */
    --table-radius: 0.5rem;

    /* Shadows */
    --shadow-100: 0px 4px 8px 0px rgba(0, 0, 0, 0.16);
    --shadow-200: 0px 8px 8px 0px rgba(0, 0, 0, 0.08);

    /* Theme switcher */
    --clr-theme-switcher-bg: var(--color-gray-100);
    --clr-theme-switcher-text: var(--color-black);

    --clr-success: var(--color-green-500);
    --clr-on-sucess: var(--color-white);

    /* Badges */
    --clr-badge-pink: oklch(59.16% 0.218 0.58 / 8%);
    --clr-badge-yellow: oklch(80.13% 0.1458 73.41 / 8%);
    --clr-badge-green: oklch(78.02% 0.1534 168.27 / 8%);
    --clr-badge-blue: oklch(82.1% 0.099263 240.9782 / 8%);
    --clr-badge-purple: oklch(52.06% 0.2042 305.37 / 8%);
    --clr-badge-gray: var(--color-gray-100);

    /* Easing */
    --ease-1: cubic-bezier(.25, 0, .5, 1);
    --ease-2: cubic-bezier(.25, 0, .4, 1);
    --ease-3: cubic-bezier(.25, 0, .3, 1);
    --ease-elastic-in-out-2: cubic-bezier(.5, -.3, .1, 1.5);
    --ease-elastic-in-out-3: cubic-bezier(.5, -.5, .1, 1.5);
    --ease-squish-2: var(--ease-elastic-in-out-2);
    --ease-squish-3: var(--ease-elastic-in-out-3);

    /* Animation */
    --animation-slide-in-up: slide-in-up 500ms var(--ease-3);
    --animation-slide-in-right: slide-in-right 500ms var(--ease-3);
    --animation-slide-in-left: slide-in-left 500ms var(--ease-3);
    --animation-slide-in-down: slide-in-down 500ms var(--ease-3);
    --animation-scale-down: scale-down 500ms var(--ease-3);
    --animation-slide-out-down: slide-out-down 500ms var(--ease-3);
    --animation-slide-out-right: slide-out-right 500ms var(--ease-3);
    --animation-slide-out-left: slide-out-left 500ms var(--ease-3);
  }

  /* Dark mode - unchanged */
  html.dark {
    color-scheme: dark;
    /* Surface */
    --clr-surface-50: var(--color-black-950);
    --clr-surface-100: var(--color-black-950);
    --clr-surface-300: var(--color-black);
    --clr-surface-400: var(--color-black-950);
    --clr-surface-500: var(--color-black);
    --clr-surface-600: var(--color-black-950);

    /* Badges */
    --clr-badge-pink: oklch(58.93% 0.2129 359.93 / 8%);
    --clr-badge-green: oklch(64.33% 0.1128 171.09 / 8%);
    --clr-badge-blue: oklch(64.12% 0.0638 240.72 / 8%);
    --clr-badge-gray: var(--color-gray-700);

    /* Text */
    --clr-text-100: var(--color-white);
    --clr-text-200: var(--color-gray-100);
    --clr-text-300: var(--color-gray-300);
    --clr-text-green: var(--color-green-600);
    --clr-text-pink: var(--color-pink-600);
    --clr-text-blue: var(--color-blue-600);

    /* Avatar */
    --clr-avatar-text: var(--color-brand-400);

    /* Border */
    --clr-border-primary: var(--color-black-800);
    --clr-border-secondary: var(--color-black-950);
    --clr-border-green: var(--color-green-600);
    --clr-border-pink: var(--color-pink-600);
    --clr-border-blue: var(--color-blue-600);

    /* Secondary btn */
    --clr-secondary-btn-bg: var(--color-black-950);
    --clr-secondary-btn-bg-active: var(--color-black);
    --clr-secondary-btn-bg-hover: var(--color-black);
    --clr-secondary-btn-text: var(--color-gray-100);
    --clr-secondary-btn-text-hover: var(--color-white);
    --clr-secondary-btn-border: var(--color-black-800);
    --clr-secondary-btn-border-hover: var(--color-black-950);

    /* Form control */
    --clr-form-control-bg: var(--color-black);
    --clr-form-control-bg-active: var(--color-black);
    --clr-form-control-placeholder: var(--color-gray-100);
    --clr-form-control-border: var(--color-black-800);
    --clr-form-control-border-hover: var(--color-brand-500);
    --clr-form-control-text: var(--color-white);
    --clr-form-control-ring: var(--color-brand-650);
    --clr-form-control-label: var(--color-white);
  }

  /* Selection - use new color variables */
  ::selection {
    background-color: var(--color-brand-600);
    color: var(--color-white);
  }

  ::popover-open {
    width: 200px;
    height: 100px;
    position: absolute;
    inset: unset;
    bottom: 5px;
    right: 5px;
    margin: 0;
  }

  button {
    cursor: default;
  }

  button:disabled {
    opacity: 0.7;
    pointer-events: none;
  }

  details>summary {
    list-style: none;
  }

  details summary::-webkit-details-marker {
    display: none;
  }

  details summary::marker {
    display: none;
  }

  input::-webkit-outer-spin-button,
  input::-webkit-inner-spin-button {
    -webkit-appearance: none;
    margin: 0;
  }

  input[type="number"] {
    -moz-appearance: textfield;
  }

  form:invalid .btn:not(.btn-enabled) {
    --events: none;
    --opacity: 0.7;
  }
}

/* Hide Alpine.js elements during load */
[x-cloak] {
  display: none !important;
}

/* Components layer - UNCHANGED */
@layer components {
  .form-control {
    --bg-color: var(--clr-form-control-bg);
    --bg-active: var(--clr-form-control-bg-active);
    --placeholder: var(--clr-form-control-placeholder);
    --text-color: var(--clr-form-control-text);
    --border-color: var(--clr-form-control-border);
    --border-hover: var(--clr-form-control-border-hover);
    --border-size: var(--form-control-border-size);
    --ring-color: var(--transparent);
    --ring-color-active: var(--clr-form-control-ring);
    --size-x: var(--form-control-size-x);
    --size-y: var(--form-control-size-y);
    --font-size: 0.875rem;
    --radius: 0.5rem;
    --opacity: 1;
    --events: all;
    pointer-events: var(--events);
    outline: none;
    background-color: oklch(var(--bg-color));
    box-shadow: 0 0 0 2px oklch(var(--ring-color));
    color: oklch(var(--text-color));
    font-size: var(--font-size);
    border-radius: var(--radius);
    opacity: var(--opacity);
    border: var(--border-size) solid oklch(var(--border-color));
    transition-duration: 200ms;
    font-weight: 500;
  }

  /* ... rest of component classes unchanged ... */
  
  .btn {
    /* Exactly as before */
  }
  
  .btn-primary {
    /* Exactly as before */
  }
  
  /* ... all other components ... */
}

/* Utilities layer - UNCHANGED */
@layer utilities {
  .hide-scrollbar {
    scrollbar-width: none;
  }

  .hide-scrollbar::-webkit-scrollbar {
    display: none;
  }

  /* ... rest of utilities ... */
}

/* Keyframes - UNCHANGED */
@keyframes slide-in-right {
  from {
    transform: translateX(100%)
  }
}

@keyframes slide-out-right {
  to {
    transform: translateX(100%)
  }
}

/* ... rest of keyframes ... */
```

---

## File 2: tailwind.config.js

### BEFORE (v3)

```javascript
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./modules/**/templates/**/*.{html,js,templ}",
    "./components/**/*.{html,js,templ,go}",
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ["Gilroy"]
      },
      backgroundColor: {
        surface: {
          100: "oklch(var(--clr-surface-100))",
          200: "oklch(var(--clr-surface-200))",
          300: "oklch(var(--clr-surface-300))",
          400: "oklch(var(--clr-surface-400))",
          500: "oklch(var(--clr-surface-500))",
          600: "oklch(var(--clr-surface-600))",
        },
        avatar: "oklch(var(--clr-avatar-bg))",
      },
      borderColor: {
        primary: "oklch(var(--clr-border-primary))",
        secondary: "oklch(var(--clr-border-secondary))",
        green: "oklch(var(--clr-border-green))",
        pink: "oklch(var(--clr-border-pink))",
        yellow: "oklch(var(--clr-border-yellow))",
        blue: "oklch(var(--clr-border-blue))",
        purple: "oklch(var(--clr-border-purple))",
      },
      textColor: {
        100: "oklch(var(--clr-text-100))",
        200: "oklch(var(--clr-text-200))",
        300: "oklch(var(--clr-text-300))",
        green: "oklch(var(--clr-text-green))",
        pink: "oklch(var(--clr-text-pink))",
        yellow: "oklch(var(--clr-text-yellow))",
        blue: "oklch(var(--clr-text-blue))",
        purple: "oklch(var(--clr-text-purple))",
        avatar: "oklch(var(--clr-avatar-text))",
      },
      colors: {
        100: "oklch(var(--clr-text-100))",
        200: "oklch(var(--clr-text-200))",
        300: "oklch(var(--clr-text-300))",
        green: "oklch(var(--clr-text-green))",
        pink: "oklch(var(--clr-text-pink))",
        yellow: "oklch(var(--clr-text-yellow))",
        blue: "oklch(var(--clr-text-blue))",
        purple: "oklch(var(--clr-text-purple))",
        avatar: "oklch(var(--clr-avatar-text))",
        black: {
          DEFAULT: "oklch(var(--black))",
          950: "oklch(var(--black-950))",
        },
        brand: {
          500: "oklch(var(--primary-500) / <alpha-value>)",
          600: "oklch(var(--primary-600) / <alpha-value>)",
          700: "oklch(var(--primary-700) / <alpha-value>)",
        },
        gray: {
          50: "oklch(var(--gray-50) / <alpha-value>)",
          100: "oklch(var(--gray-100) / <alpha-value>)",
          200: "oklch(var(--gray-200) / <alpha-value>)",
          300: "oklch(var(--gray-300) / <alpha-value>)",
          400: "oklch(var(--gray-400) / <alpha-value>)",
          500: "oklch(var(--gray-500) / <alpha-value>)",
          600: "oklch(var(--gray-600) / <alpha-value>)",
          700: "oklch(var(--gray-700) / <alpha-value>)",
          800: "oklch(var(--gray-800) / <alpha-value>)",
          900: "oklch(var(--gray-900) / <alpha-value>)",
          950: "oklch(var(--gray-950) / <alpha-value>)",
        },
        green: {
          50: "oklch(var(--green-50) / <alpha-value>)",
          100: "oklch(var(--green-100) / <alpha-value>)",
          200: "oklch(var(--green-200) / <alpha-value>)",
          500: "oklch(var(--green-500) / <alpha-value>)",
          600: "oklch(var(--green-600) / <alpha-value>)",
        },
        red: {
          100: "oklch(var(--red-100))",
          200: "oklch(var(--red-200))",
          500: "oklch(var(--red-500) / <alpha-value>)",
          600: "oklch(var(--red-600) / <alpha-value>)",
          700: "oklch(var(--red-700) / <alpha-value>)",
        },
        badge: {
          pink: "oklch(var(--clr-badge-pink))",
          yellow: "oklch(var(--clr-badge-yellow))",
          green: "oklch(var(--clr-badge-green))",
          blue: "oklch(var(--clr-badge-blue))",
          purple: "oklch(var(--clr-badge-purple))",
          gray: "oklch(var(--clr-badge-gray))",
        },
        success: {
          DEFAULT: "oklch(var(--green-500) / <alpha-value>)"
        },
        on: {
          success: "oklch(var(--white))"
        }
      },
    },
  },
  plugins: [],
}
```

### AFTER (v4)

**File is deleted!** All configuration moved to CSS.

If you want to keep it as a backup:
```bash
mv tailwind.config.js tailwind.config.js.v3.backup
```

---

## File 3: Makefile

### BEFORE (v3)

```makefile
# Variables
TAILWIND_INPUT := modules/core/presentation/assets/css/main.css
TAILWIND_OUTPUT := modules/core/presentation/assets/css/main.min.css

# CSS build
css:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "watch" ]; then \
		tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify --watch; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "dev" ]; then \
		tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT); \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "clean" ]; then \
		rm -rf $(TAILWIND_OUTPUT); \
	else \
		tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify; \
	fi
```

### AFTER (v4)

```makefile
# Variables
TAILWIND_INPUT := modules/core/presentation/assets/css/main.css
TAILWIND_OUTPUT := modules/core/presentation/assets/css/main.min.css

# CSS build
css:
	@if [ "$(word 2,$(MAKECMDGOALS))" = "watch" ]; then \
		tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify --watch; \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "dev" ]; then \
		tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT); \
	elif [ "$(word 2,$(MAKECMDGOALS))" = "clean" ]; then \
		rm -rf $(TAILWIND_OUTPUT); \
	else \
		tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify; \
	fi
```

**Changes**: Removed `-c tailwind.config.js` flag from all commands.

---

## File 4: ai-chat/app/globals.css

### BEFORE (v3)

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  --background: 0 0% 100%;
  --foreground: 222.2 84% 4.9%;
  --primary: 214 60% 44%;
  --primary-foreground: 210 40% 98%;
}

body {
  font-family: "Inter", sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

@layer utilities {
  .text-balance {
    text-wrap: balance;
  }
}

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 0 0% 3.9%;
    --card: 0 0% 100%;
    --card-foreground: 0 0% 3.9%;
    /* ... many more HSL tokens ... */
  }
  .dark {
    --background: 0 0% 3.9%;
    --foreground: 0 0% 98%;
    /* ... dark mode tokens ... */
  }
}

@layer base {
  * {
    @apply border-border;
  }
  body {
    @apply bg-background text-foreground;
  }
}

/* Markdown content styling */
.markdown-content {
  /* ... custom classes ... */
}
```

### AFTER (v4)

```css
@import "tailwindcss";

/* Content sources */
@source "../pages/**/*.{js,ts,jsx,tsx,mdx}";
@source "../components/**/*.{js,ts,jsx,tsx,mdx}";
@source "../app/**/*.{js,ts,jsx,tsx,mdx}";

/* Theme tokens */
@theme {
  /* Can keep HSL or convert to OKLCH - both work! */
  /* Using OKLCH for consistency with main project: */
  
  --color-background: oklch(100% 0 0);  /* white */
  --color-foreground: oklch(20.5% 0.028 264.665);  /* ~#0f0f0f */
  
  --color-card: oklch(100% 0 0);
  --color-card-foreground: oklch(20.5% 0.028 264.665);
  
  --color-popover: oklch(100% 0 0);
  --color-popover-foreground: oklch(20.5% 0.028 264.665);
  
  --color-primary: oklch(54% 0.14 250);  /* ~#2e67b4 */
  --color-primary-foreground: oklch(98% 0.002 247);
  
  --color-secondary: oklch(97% 0.003 264);
  --color-secondary-foreground: oklch(20% 0.028 265);
  
  --color-muted: oklch(97% 0.003 264);
  --color-muted-foreground: oklch(49% 0.027 264);
  
  --color-accent: oklch(97% 0.003 264);
  --color-accent-foreground: oklch(20% 0.028 265);
  
  --color-destructive: oklch(65% 0.22 20);  /* red */
  --color-destructive-foreground: oklch(98% 0.002 247);
  
  --color-border: oklch(91% 0.006 264);
  --color-input: oklch(91% 0.006 264);
  --color-ring: oklch(20.5% 0.028 264.665);
  
  /* Charts */
  --color-chart-1: oklch(69% 0.18 30);
  --color-chart-2: oklch(52% 0.09 190);
  --color-chart-3: oklch(40% 0.08 210);
  --color-chart-4: oklch(75% 0.16 80);
  --color-chart-5: oklch(75% 0.18 50);
  
  /* Sidebar */
  --color-sidebar-background: oklch(98% 0.002 247);
  --color-sidebar-foreground: oklch(39% 0.03 256);
  --color-sidebar-primary: oklch(23% 0.028 262);
  --color-sidebar-primary-foreground: oklch(98% 0.002 247);
  --color-sidebar-accent: oklch(96.5% 0.003 264);
  --color-sidebar-accent-foreground: oklch(23% 0.028 262);
  --color-sidebar-border: oklch(92% 0.015 230);
  --color-sidebar-ring: oklch(68% 0.26 252);
  
  /* Radius */
  --radius: 0.5rem;
}

body {
  font-family: "Inter", sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

@layer utilities {
  .text-balance {
    text-wrap: balance;
  }
}

@layer base {
  /* Dark mode overrides */
  .dark {
    --color-background: oklch(20.5% 0.028 264.665);
    --color-foreground: oklch(98% 0.002 247);
    
    --color-card: oklch(20.5% 0.028 264.665);
    --color-card-foreground: oklch(98% 0.002 247);
    
    --color-popover: oklch(20.5% 0.028 264.665);
    --color-popover-foreground: oklch(98% 0.002 247);
    
    --color-primary: oklch(98% 0.002 247);
    --color-primary-foreground: oklch(20% 0.028 265);
    
    --color-secondary: oklch(27% 0.033 256);
    --color-secondary-foreground: oklch(98% 0.002 247);
    
    --color-muted: oklch(27% 0.033 256);
    --color-muted-foreground: oklch(66% 0.027 264);
    
    --color-accent: oklch(27% 0.033 256);
    --color-accent-foreground: oklch(98% 0.002 247);
    
    --color-destructive: oklch(50% 0.18 20);
    --color-destructive-foreground: oklch(98% 0.002 247);
    
    --color-border: oklch(27% 0.033 256);
    --color-input: oklch(27% 0.033 256);
    --color-ring: oklch(84% 0.028 261);
    
    /* Charts - dark mode */
    --color-chart-1: oklch(60% 0.16 230);
    --color-chart-2: oklch(52% 0.10 175);
    --color-chart-3: oklch(60% 0.14 45);
    --color-chart-4: oklch(65% 0.14 290);
    --color-chart-5: oklch(70% 0.16 350);
    
    /* Sidebar - dark mode */
    --color-sidebar-background: oklch(23% 0.028 262);
    --color-sidebar-foreground: oklch(96.5% 0.003 264);
    --color-sidebar-primary: oklch(64% 0.22 252);
    --color-sidebar-primary-foreground: oklch(100% 0 0);
    --color-sidebar-accent: oklch(28% 0.033 256);
    --color-sidebar-accent-foreground: oklch(96.5% 0.003 264);
    --color-sidebar-border: oklch(28% 0.033 256);
    --color-sidebar-ring: oklch(68% 0.26 252);
  }
  
  * {
    @apply border-border;
  }
  body {
    @apply bg-background text-foreground;
  }
}

/* Markdown content styling - UNCHANGED */
.markdown-content {
  font-size: inherit;
  line-height: 1.5;
}

/* ... rest of markdown styles unchanged ... */
```

---

## File 5: ai-chat/tailwind.config.ts

### BEFORE (v3)

```typescript
import type { Config } from "tailwindcss"

const config: Config = {
  darkMode: ["class"],
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        "dark-blue": "#0a223e",
        "light-bg": "#f2f5f8",
        /* ... many colors ... */
      },
      /* ... rest of theme ... */
    },
  },
  plugins: [],
}
export default config
```

### AFTER (v4)

**File is deleted!** All configuration moved to `app/globals.css`.

Backup:
```bash
mv ai-chat/tailwind.config.ts ai-chat/tailwind.config.ts.v3.backup
```

---

## File 6: ai-chat/postcss.config.js

### BEFORE (v3)

```javascript
module.exports = {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
```

### AFTER (v4)

```javascript
// postcss.config.mjs
export default {
  plugins: {
    "@tailwindcss/postcss": {},
  }
}
```

**Changes**:
- Rename file from `.js` to `.mjs`
- Use ES6 export syntax
- Use `@tailwindcss/postcss` plugin
- Remove `autoprefixer` (built-in to v4)

---

## File 7: ai-chat/package.json

### BEFORE (v3)

```json
{
  "devDependencies": {
    "tailwindcss": "^3.4.17",
    "postcss": "^8",
    "postcss-cli": "^11.0.0",
    "autoprefixer": "^10.4.20"
  }
}
```

### AFTER (v4)

```json
{
  "devDependencies": {
    "tailwindcss": "^4.0.0",
    "@tailwindcss/postcss": "^4.0.0",
    "postcss": "^8",
    "postcss-cli": "^11.0.0"
  }
}
```

**Install**:
```bash
cd ai-chat
npm uninstall autoprefixer
npm install tailwindcss@next @tailwindcss/postcss@next
```

---

## Summary of Changes

| File | Action |
|------|--------|
| `modules/core/presentation/assets/css/main.css` | Major refactor |
| `tailwind.config.js` | **Delete** (move config to CSS) |
| `ai-chat/app/globals.css` | Major refactor |
| `ai-chat/tailwind.config.ts` | **Delete** (move config to CSS) |
| `ai-chat/postcss.config.js` | Rename to `.mjs`, update plugin |
| `ai-chat/package.json` | Update dependencies |
| `Makefile` | Remove `-c tailwind.config.js` flag |

---

**Key Principle**: Move all theme configuration from JavaScript to CSS `@theme` blocks.
