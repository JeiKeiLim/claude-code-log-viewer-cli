#!/bin/bash
# story-pipeline.sh - Automates the full BMAD story workflow
# Usage: ./scripts/story-pipeline.sh <story-id>
# Example: ./scripts/story-pipeline.sh 3-5-1-directory-manager

set -e  # Exit on first error

STORY=$1

if [ -z "$STORY" ]; then
    echo "Usage: $0 <story-id>"
    echo "Example: $0 3-5-1-directory-manager"
    exit 1
fi

echo "========================================"
echo "🚀 Starting story pipeline for: $STORY"
echo "========================================"

# Step 1: Create Story
echo ""
echo "📝 Step 1/4: Creating story..."
echo "----------------------------------------"
claude -p "/bmad:bmm:agents:sm create story $STORY"

# Step 2: Validate Story
echo ""
echo "✅ Step 2/4: Validating story..."
echo "----------------------------------------"
claude -p "/bmad:bmm:agents:sm *validate-create-story $STORY then apply all suggested improvements without asking me."

# Step 3: Dev Story
echo ""
echo "🔨 Step 3/4: Implementing story..."
echo "----------------------------------------"
claude -p "/bmad:bmm:agents:dev *dev-story $STORY do not ask and proceed right away"

# Step 4: Code Review
echo ""
echo "🔍 Step 4/4: Code review..."
echo "----------------------------------------"
claude -p "/bmad:bmm:agents:dev 1. *code-review $STORY 2. apply all fixes.  3. update sprint status 4. commit. Must follow the order."

echo ""
echo "========================================"
echo "✓ Story $STORY pipeline complete!"
echo "========================================"
