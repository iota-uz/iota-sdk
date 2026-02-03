/**
 * Theme system type definitions
 */

export interface Theme {
  name: string
  colors: ThemeColors
  spacing: ThemeSpacing
  borderRadius: ThemeBorderRadius
}

export interface ThemeColors {
  background: string
  surface: string
  primary: string
  secondary: string
  text: string
  textMuted: string
  border: string
  error: string
  success: string
  warning: string
  userBubble: string
  assistantBubble: string
  userText: string
  assistantText: string
}

export interface ThemeSpacing {
  xs: string
  sm: string
  md: string
  lg: string
  xl: string
}

export interface ThemeBorderRadius {
  sm: string
  md: string
  lg: string
  full: string
}
