---
allowed-tools: mcp__sequential-thinking__sequentialthinking, Read, Grep, Glob, LS, 
  mcp__bloom__search_code, Task, TodoWrite
description: Design software architecture using Tree of Thought approach
argument-hint: <design-requirement>
---

# Architect Mode: Design $ARGUMENTS

You are now in architect mode. Your goal is to design interfaces, methods, and types WITHOUT implementing the actual function bodies. Focus on creating clean, well-thought-out architectures.

## Design Process

### Phase 1: Understanding & Analysis
Use the sequential thinking tool to:
1. Analyze the requirement: "$ARGUMENTS"
2. Identify key components and their relationships
3. Consider constraints and non-functional requirements
4. Research existing patterns in the codebase using `mcp__bloom__search_code`

### Phase 2: Tree of Thought - Generate 3 Design Solutions
Use sequential thinking to explore three different architectural approaches. For each solution, explicitly consider how it adheres to SOLID, DRY, KISS, and GRASP principles.

#### Solution 1: [Name your first approach]
- Design philosophy
- Key interfaces and types
- Method signatures
- Principle adherence:
  - How it follows SOLID
  - How it implements GRASP patterns
  - DRY and KISS considerations
- Trade-offs

#### Solution 2: [Name your second approach]
- Design philosophy
- Key interfaces and types
- Method signatures
- Principle adherence:
  - How it follows SOLID
  - How it implements GRASP patterns
  - DRY and KISS considerations
- Trade-offs

#### Solution 3: [Name your third approach]
- Design philosophy
- Key interfaces and types
- Method signatures
- Principle adherence:
  - How it follows SOLID
  - How it implements GRASP patterns
  - DRY and KISS considerations
- Trade-offs

### Phase 3: Convergence
Use sequential thinking to:
1. Compare the three solutions
2. Identify the best aspects of each
3. Converge on the optimal design
4. Refine the chosen solution

## Design Output Format

### Interfaces
```go
// Repository interface for [Entity]
type [Entity]Repository interface {
    // Method signatures only - no implementation
    Create(ctx context.Context, entity *[Entity]) error
    FindByID(ctx context.Context, id string) (*[Entity], error)
    // ... other methods
}
```

### Domain Types
```go
// [Entity] represents [description]
type [Entity] struct {
    ID        string
    // ... fields
}

// Value objects
type [ValueObject] struct {
    // ... fields
}
```

### Service Interfaces
```go
// [Entity]Service handles business logic for [Entity]
type [Entity]Service interface {
    // Method signatures with clear intent
    Process[Action](ctx context.Context, params [Params]) ([Result], error)
    // ... other methods
}
```

### DTOs and ViewModels
```go
// Request/Response types
type Create[Entity]Request struct {
    // ... fields
}

type [Entity]ViewModel struct {
    // ... fields for presentation layer
}
```

## Design Principles

### SOLID Principles
- **S**ingle Responsibility: Each component has one reason to change
- **O**pen/Closed: Open for extension, closed for modification
- **L**iskov Substitution: Subtypes must be substitutable for base types
- **I**nterface Segregation: Many specific interfaces over general ones
- **D**ependency Inversion: Depend on abstractions, not concretions

### Additional Principles
- **DRY** (Don't Repeat Yourself): Single source of truth
- **KISS** (Keep It Simple, Stupid): Simplest solution that works
- **Domain-Driven Design** (DDD): Model the business domain

### GRASP Principles
- **Creator**: Assign creation responsibility appropriately
- **Information Expert**: Assign responsibility to the class with the information
- **Low Coupling**: Minimize dependencies between classes
- **High Cohesion**: Keep related functionality together
- **Controller**: Coordinate and delegate work
- **Polymorphism**: Use polymorphism for type-based variations
- **Pure Fabrication**: Create artificial classes when needed
- **Indirection**: Use intermediary to reduce coupling
- **Protected Variations**: Shield elements from variations

## Architecture Considerations
- Error handling patterns (use pkg/serrors)
- Event-driven capabilities
- Testing strategy
- Performance implications
- Security considerations

## Module Structure
Show how the design fits into the module architecture:
```
modules/{module}/
├── domain/
│   ├── aggregates/
│   ├── entities/
│   └── value_objects/
├── infrastructure/
├── services/
└── presentation/
```

## Important Notes
- DO NOT implement function bodies
- DO NOT write actual logic
- Focus on contracts and interfaces
- Consider extensibility and maintainability
- Document design decisions and rationale
- Show relationships between components