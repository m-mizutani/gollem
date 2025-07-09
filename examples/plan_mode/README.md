# Plan Mode Example

This example demonstrates gollem's plan mode functionality, which breaks down complex tasks into structured steps and executes them with intelligent adaptation capabilities.

## Features Demonstrated

### 1. Multi-Step Planning
- Automatic breakdown of complex goals into actionable steps
- Dynamic plan adaptation based on execution results
- Comprehensive progress tracking and monitoring

### 2. Adaptive Skip Functionality
The example showcases three different execution modes:

#### Complete Mode
- **Behavior**: Executes all planned tasks without skipping
- **Use Case**: When thoroughness is more important than efficiency
- **Example**: Comprehensive research projects requiring all steps

#### Balanced Mode (Default)
- **Behavior**: Allows skipping with confidence-based confirmation
- **Use Case**: Most common scenarios balancing efficiency and completeness
- **Default Settings**: 
  - Execution mode: `PlanExecutionModeBalanced`
  - Confidence threshold: `0.8` (80%)
  - Skip confirmation: Auto-approve if confidence >= threshold
- **Features**: 
  - Requires confidence threshold (default: 0.8)
  - Custom confirmation hooks for user interaction
  - Transparent skip reasoning with evidence

#### Efficient Mode
- **Behavior**: Aggressively skips tasks when confidence threshold is met
- **Use Case**: Time-sensitive tasks where speed is prioritized
- **Features**:
  - Lower confidence thresholds (e.g., 0.6-0.7)
  - Automatic approval of skip decisions
  - Optimized for rapid completion

### 3. Skip Decision Intelligence
The LLM provides structured skip decisions with:
- **Confidence Levels**: 0.0-1.0 scale indicating certainty
- **Detailed Reasoning**: Clear explanation of why a task should be skipped
- **Evidence**: Supporting information from previous execution results
- **Transparency**: Full visibility into decision-making process

## Tools Used

- **SearchTool**: Simulates web search functionality for research
- **AnalysisTool**: Performs data analysis with configurable types
- **ReportTool**: Generates formatted reports from analysis results

## Running the Example

```bash
# Set your OpenAI API key
export OPENAI_API_KEY="your-api-key-here"

# Run the example
go run main.go
```

## Example Output

The demo will show three different execution modes:

1. **Complete Mode**: All tasks executed regardless of redundancy
2. **Balanced Mode**: Smart skipping with confirmation and reasoning
3. **Efficient Mode**: Aggressive skipping for rapid completion

Each mode displays:
- Plan creation with task breakdown
- Real-time execution progress
- Skip decisions with confidence levels and reasoning
- Final summary with efficiency metrics

## Skip Decision Examples

```
🤔 Skip decision (confidence: 0.85):
   Reason: Analysis already completed in previous step with comprehensive results
   Evidence: Step 2 output contains detailed trend analysis with 15 key insights
   → Approving for demo

⏭️  Skipped: Validate analysis results
```

## Configuration Options

### Execution Modes
```go
gollem.WithPlanExecutionMode(gollem.PlanExecutionModeComplete)   // No skipping
gollem.WithPlanExecutionMode(gollem.PlanExecutionModeBalanced)   // Default adaptive (DEFAULT)
gollem.WithPlanExecutionMode(gollem.PlanExecutionModeEfficient)  // Aggressive skipping
```

### Skip Thresholds
```go
gollem.WithSkipConfidenceThreshold(0.8)  // Default: 80% confidence required
gollem.WithSkipConfidenceThreshold(0.6)  // Lower threshold for efficiency
gollem.WithSkipConfidenceThreshold(0.9)  // Higher threshold for thoroughness
```

### Custom Confirmation
```go
// Default behavior: Auto-approve if confidence >= threshold (0.8)
gollem.WithSkipConfirmationHook(func(ctx context.Context, plan *gollem.Plan, decision gollem.SkipDecision) bool {
    // Custom logic for skip approval
    return decision.Confidence >= 0.9
})
```

### Using Defaults
```go
// Equivalent to: PlanExecutionModeBalanced + 0.8 threshold + default confirmation
plan, err := agent.Plan(ctx, "task description")
```

## Key Benefits

1. **Efficiency**: Avoid redundant work when goals are already achieved
2. **Transparency**: Clear reasoning for all skip decisions
3. **Flexibility**: Multiple execution modes for different use cases
4. **Intelligence**: LLM learns from execution results to make better decisions
5. **Control**: Custom confirmation hooks for specific requirements

This example demonstrates how gollem's plan mode can intelligently adapt execution based on intermediate results, making AI agents more efficient and practical for real-world tasks.