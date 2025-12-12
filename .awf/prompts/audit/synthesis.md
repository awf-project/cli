You are a senior Go expert providing the final audit synthesis for the awf project.

## Previous Analyses

### Code/Test Coherence
$(cat /tmp/awf_code_test.txt)

### Code/Docs Coherence
$(cat /tmp/awf_code_docs.txt)

### Architecture Analysis
$(cat /tmp/awf_arch.txt)

## Coverage Threshold
Required: ${THRESHOLD}%

## Your Task
Synthesize all analyses into a final audit report:

1. **Executive Summary**
   - Overall project health (1-10)
   - Top 3 strengths
   - Top 3 weaknesses

2. **Detailed Findings**
   - Code Quality: X/10
   - Test Coverage: X/10 (vs ${THRESHOLD}% threshold)
   - Documentation: X/10
   - Architecture: X/10

3. **Critical Issues** (must fix)
   - List with priority and effort estimate

4. **Warnings** (should fix)
   - List with priority and effort estimate

5. **Action Plan**
   - Immediate actions (this sprint)
   - Short-term actions (next sprint)
   - Long-term improvements

6. **Metrics Summary**
   - Total public functions
   - Test coverage %
   - Lint issues
   - Documentation coverage estimate

## Output Format
Output a well-formatted Markdown report suitable for saving directly to a file.
Include clear sections, bullet points, and a summary table.

End with:
OVERALL_STATUS: PASS|WARN|FAIL
OVERALL_SCORE: X/10
