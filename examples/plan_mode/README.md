# Plan Mode Example

This example demonstrates the Plan-and-Execute functionality in gollem.

## What it does

1. **Creates a Plan**: The agent breaks down a complex research task into logical steps
2. **Serializes the Plan**: Shows how plans can be saved and restored for asynchronous processing
3. **Executes the Plan**: Runs each step of the plan using the appropriate tools

## Features Demonstrated

- **Plan Creation**: `Agent.Plan()` method with custom options
- **Serialization**: `Plan.Serialize()` and `Agent.NewPlanFromData()` methods
- **Plan Execution**: `Plan.Execute()` method that runs the complete plan
- **Tool Integration**: Custom tools (search, analysis) used during plan execution
- **System Prompts**: Custom system prompts for different phases of execution

## Running the Example

1. Set your OpenAI API key:
   ```bash
   export OPENAI_API_KEY=your_api_key_here
   ```

2. Run the example:
   ```bash
   go run examples/plan_mode/main.go
   ```

## Expected Output

The example will:
1. Create a multi-step plan for researching renewable energy
2. Serialize the plan to show persistence capability 
3. Deserialize and execute the plan
4. Display the final research analysis result

## Key Concepts

### Plan vs Execute
- **Plan**: Creates a strategic breakdown of the task without executing it
- **Execute**: Runs the actual plan step by step, using tools as needed

### Serialization
Plans can be serialized to JSON for:
- Saving to databases
- Passing between services
- Asynchronous execution approval workflows

### System Prompts
Different system prompts can be used for:
- Planning phase: Strategic thinking and task breakdown
- Execution phase: Tool selection and task completion