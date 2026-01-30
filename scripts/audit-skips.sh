#!/bin/bash
# audit-skips.sh
#
# Purpose: Categorize all t.Skip() calls in test files by skip reason pattern
# Usage: ./scripts/audit-skips.sh
# Output: Categorized report of skip patterns with file locations and counts

set -euo pipefail

# Color codes for output
readonly COLOR_RESET='\033[0m'
readonly COLOR_BOLD='\033[1m'
readonly COLOR_CYAN='\033[0;36m'
readonly COLOR_YELLOW='\033[0;33m'
readonly COLOR_GREEN='\033[0;32m'

# Category patterns (order matters - more specific first)
declare -A CATEGORIES=(
    ["integration"]="skipping integration test"
    ["not_implemented"]="not yet implemented"
    ["cli_tool"]="CLI not installed|not installed|gemini.*not|codex.*not|claude.*not|opencode.*not"
    ["platform"]="platform|symlink|windows|linux|darwin"
    ["root_user"]="root|sudo|permission"
    ["short_mode"]="short|slow|resource"
    ["fixture"]="fixture|directory not created"
    ["pending"]="pending|todo|fixme|will be tested|documents that"
    ["stub"]="stub|negative test|edge case"
    ["other"]=".*"
)

# Global counters
declare -A CATEGORY_COUNTS
declare -A CATEGORY_FILES
TOTAL_SKIPS=0

main() {
    echo -e "${COLOR_BOLD}=== Test Skip Audit Report ===${COLOR_RESET}"
    echo -e "Generated: $(date)\n"

    # Initialize category counts
    for category in "${!CATEGORIES[@]}"; do
        CATEGORY_COUNTS[$category]=0
        CATEGORY_FILES[$category]=""
    done

    # Create temp file for grep output
    local temp_file
    temp_file=$(mktemp)
    trap "rm -f $temp_file" EXIT

    # Get all skip calls
    grep -rn "t\.Skip(" --include="*_test.go" . 2>/dev/null > "$temp_file" || true

    if [[ ! -s "$temp_file" ]]; then
        echo "No t.Skip() calls found in test files."
        return 0
    fi

    # Process each line
    while IFS= read -r line || [[ -n "$line" ]]; do
        # Skip empty lines
        if [[ -z "$line" ]]; then
            continue
        fi

        TOTAL_SKIPS=$((TOTAL_SKIPS + 1))

        # Split file:line:content
        local file line_num skip_line
        file=$(echo "$line" | cut -d: -f1)
        line_num=$(echo "$line" | cut -d: -f2)
        skip_line=$(echo "$line" | cut -d: -f3-)

        # Extract skip message (between quotes)
        local skip_msg
        skip_msg=$(echo "$skip_line" | sed -n 's/.*t\.Skip("\([^"]*\)".*/\1/p')

        # Categorize the skip
        local category
        category=$(categorize_skip "$skip_msg")

        # Update counters
        CATEGORY_COUNTS[$category]=$((${CATEGORY_COUNTS[$category]} + 1))

        # Store file reference (limit to first 100 per category)
        local current_count
        if [[ -z "${CATEGORY_FILES[$category]}" ]]; then
            current_count=0
        else
            current_count=$(echo -n "${CATEGORY_FILES[$category]}" | wc -l)
        fi

        if [[ $current_count -lt 100 ]]; then
            if [[ -z "${CATEGORY_FILES[$category]}" ]]; then
                CATEGORY_FILES[$category]="$file:$line_num"
            else
                CATEGORY_FILES[$category]="${CATEGORY_FILES[$category]}"$'\n'"$file:$line_num"
            fi
        fi

    done < "$temp_file"

    # Print summary
    print_summary

    # Print detailed breakdown
    print_details

    echo -e "\n${COLOR_BOLD}Total Skipped Tests: ${TOTAL_SKIPS}${COLOR_RESET}"
}

categorize_skip() {
    local msg="$1"

    # Match against patterns (order matters)
    for category in integration not_implemented cli_tool platform root_user short_mode fixture pending stub other; do
        local pattern="${CATEGORIES[$category]}"
        if [[ "$msg" =~ $pattern ]]; then
            echo "$category"
            return
        fi
    done

    echo "other"
}

print_summary() {
    echo -e "${COLOR_BOLD}Summary by Category:${COLOR_RESET}\n"
    printf "%-20s %10s %10s\n" "Category" "Count" "Percentage"
    printf "%-20s %10s %10s\n" "--------" "-----" "----------"

    # Sort by count (descending)
    for category in $(for k in "${!CATEGORY_COUNTS[@]}"; do echo "$k ${CATEGORY_COUNTS[$k]}"; done | sort -k2 -rn | cut -d' ' -f1); do
        local count=${CATEGORY_COUNTS[$category]}
        local percentage=0
        if [[ $TOTAL_SKIPS -gt 0 ]]; then
            percentage=$(( count * 100 / TOTAL_SKIPS ))
        fi

        printf "%-20s %10d %9d%%\n" "$category" "$count" "$percentage"
    done
}

print_details() {
    echo -e "\n${COLOR_BOLD}Detailed Breakdown:${COLOR_RESET}\n"

    for category in $(for k in "${!CATEGORY_COUNTS[@]}"; do echo "$k ${CATEGORY_COUNTS[$k]}"; done | sort -k2 -rn | cut -d' ' -f1); do
        local count=${CATEGORY_COUNTS[$category]}

        if [[ $count -eq 0 ]]; then
            continue
        fi

        echo -e "${COLOR_CYAN}[$category] (${count} occurrences)${COLOR_RESET}"
        echo "Pattern: ${CATEGORIES[$category]}"

        # Get unique file list
        local files
        files=$(echo "${CATEGORY_FILES[$category]}" | cut -d: -f1 | sort -u)

        local file_count
        file_count=$(echo "$files" | wc -l)

        echo "Files affected: $file_count"
        echo "Examples:"

        # Show up to 5 examples
        echo "${CATEGORY_FILES[$category]}" | head -5 | while IFS= read -r location; do
            echo "  - $location"
        done

        if [[ $(echo "${CATEGORY_FILES[$category]}" | wc -l) -gt 5 ]]; then
            local remaining=$(( $(echo "${CATEGORY_FILES[$category]}" | wc -l) - 5 ))
            echo "  ... and $remaining more"
        fi

        # Suggested action
        case "$category" in
            integration)
                echo -e "${COLOR_YELLOW}Action: Convert to //go:build integration tag${COLOR_RESET}"
                ;;
            not_implemented)
                echo -e "${COLOR_YELLOW}Action: Link to feature spec or delete if obsolete${COLOR_RESET}"
                ;;
            cli_tool)
                echo -e "${COLOR_YELLOW}Action: Convert to //go:build external or use helper${COLOR_RESET}"
                ;;
            platform|root_user)
                echo -e "${COLOR_YELLOW}Action: Create standardized skip helper function${COLOR_RESET}"
                ;;
            short_mode)
                echo -e "${COLOR_GREEN}Action: Keep - consider //go:build !short if appropriate${COLOR_RESET}"
                ;;
            pending|stub)
                echo -e "${COLOR_YELLOW}Action: Create tracking issue or complete implementation${COLOR_RESET}"
                ;;
            *)
                echo -e "${COLOR_YELLOW}Action: Review individually${COLOR_RESET}"
                ;;
        esac

        echo ""
    done
}

main "$@"
