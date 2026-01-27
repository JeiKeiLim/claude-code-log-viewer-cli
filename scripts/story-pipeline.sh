#!/bin/bash
# story-pipeline.sh - Automates the full BMAD story workflow
# Usage: ./scripts/story-pipeline.sh <story-id> [-s N] [-e N]
# Example: ./scripts/story-pipeline.sh 3-5-1-directory-manager
# Example: ./scripts/story-pipeline.sh 3-5-1-directory-manager -s 3      # Start from step 3
# Example: ./scripts/story-pipeline.sh 3-5-1-directory-manager -s 1 -e 2 # Run steps 1-2 only

set -e  # Exit on first error

STORY=""
START_STEP=1
END_STEP=4

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --step|-s)
            START_STEP="$2"
            shift 2
            ;;
        --end|-e)
            END_STEP="$2"
            shift 2
            ;;
        *)
            if [ -z "$STORY" ]; then
                STORY="$1"
            fi
            shift
            ;;
    esac
done

if [ -z "$STORY" ]; then
    echo "Usage: $0 <story-id> [-s N] [-e N]"
    echo "Example: $0 3-5-1-directory-manager"
    echo "Example: $0 3-5-1-directory-manager -s 3      # Start from step 3"
    echo "Example: $0 3-5-1-directory-manager -s 1 -e 2 # Run steps 1-2 only"
    echo ""
    echo "Steps:"
    echo "  1: Create story"
    echo "  2: Validate story"
    echo "  3: Implement story (dev-story)"
    echo "  4: Code review + commit"
    exit 1
fi

echo "========================================"
echo "Starting story pipeline for: $STORY"
if [ "$START_STEP" -gt 1 ] || [ "$END_STEP" -lt 4 ]; then
    echo "Running steps $START_STEP-$END_STEP"
fi
echo "========================================"

# Step 1: Create Story
if [ "$START_STEP" -le 1 ] && [ "$END_STEP" -ge 1 ]; then
    echo ""
    echo "Step 1/4: Creating story..."
    echo "----------------------------------------"
    claude -p "/bmad:bmm:agents:sm *create story $STORY"
fi

# Step 2: Validate Story
if [ "$START_STEP" -le 2 ] && [ "$END_STEP" -ge 2 ]; then
    echo ""
    echo "Step 2/4: Validating story..."
    echo "----------------------------------------"
    claude -p "/bmad:bmm:agents:sm *validate-create-story $STORY then apply all suggested improvements without asking me."
fi

# Step 3: Dev Story
if [ "$START_STEP" -le 3 ] && [ "$END_STEP" -ge 3 ]; then
    echo ""
    echo "Step 3/4: Implementing story..."
    echo "----------------------------------------"
    claude -p "/bmad:bmm:agents:dev *dev-story $STORY do not ask and proceed right away"
fi

# Step 4: Code Review
if [ "$START_STEP" -le 4 ] && [ "$END_STEP" -ge 4 ]; then
    echo ""
    echo "Step 4/4: Code review..."
    echo "----------------------------------------"
    claude -p "/bmad:bmm:agents:dev 1. *code-review $STORY 2. apply all fixes.  3. update sprint status 4. commit. Must follow the order."
fi

echo ""
echo "========================================"
echo "Story $STORY pipeline complete!"
echo "========================================"
