#!/bin/bash

# Validation script for @iotauz/design-tokens package

echo "🔍 Validating @iotauz/design-tokens package structure..."
echo ""

# Check required files exist
FILES=(
  "package.json"
  "README.md"
  "CHANGELOG.md"
  "EXAMPLE.md"
  "index.css"
  "theme.css"
  "base.css"
  "components.css"
  "utilities.css"
  "tokens/colors.css"
  "tokens/typography.css"
  "tokens/spacing.css"
)

MISSING=0
for file in "${FILES[@]}"; do
  if [ -f "$file" ]; then
    echo "✅ $file"
  else
    echo "❌ $file - MISSING"
    MISSING=1
  fi
done

echo ""

# Validate package.json structure
echo "📦 Validating package.json..."
if command -v jq &> /dev/null; then
  NAME=$(jq -r '.name' package.json)
  VERSION=$(jq -r '.version' package.json)
  MAIN=$(jq -r '.main' package.json)
  
  if [ "$NAME" = "@iotauz/design-tokens" ]; then
    echo "✅ Package name: $NAME"
  else
    echo "❌ Invalid package name: $NAME"
    MISSING=1
  fi
  
  if [ "$MAIN" = "index.css" ]; then
    echo "✅ Main entry: $MAIN"
  else
    echo "❌ Invalid main entry: $MAIN"
    MISSING=1
  fi
  
  echo "📌 Version: $VERSION"
else
  echo "⚠️  jq not installed, skipping JSON validation"
fi

echo ""

# Check for color tokens
echo "🎨 Checking color tokens..."
COLOR_COUNT=$(grep -c "color-" tokens/colors.css)
if [ "$COLOR_COUNT" -gt 30 ]; then
  echo "✅ Found $COLOR_COUNT color tokens"
else
  echo "⚠️  Only found $COLOR_COUNT color tokens (expected 30+)"
fi

echo ""

# Check file sizes
echo "📊 File sizes:"
ls -lh *.css tokens/*.css | awk '{print $9, $5}'

echo ""

if [ $MISSING -eq 0 ]; then
  echo "✨ Package validation passed!"
  exit 0
else
  echo "❌ Package validation failed!"
  exit 1
fi
