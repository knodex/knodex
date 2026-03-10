# Example ResourceGraphDefinitions (RGDs)

This directory contains example RGD manifests that demonstrate various KRO features and are automatically deployed when running `make qa-deploy`.

## Examples

### 1. simple-app.yaml
**Purpose**: Demonstrates basic RGD structure with template resources

**Features**:
- Basic schema definition with required and optional fields
- Template resources (Deployment, Service)
- Schema field interpolation using `${schema.spec.*}`

**Schema Fields**:
- `appName` (required): Application name
- `image`: Container image (default: nginx:latest)
- `port`: Container port (default: 80)

**Resources**:
- Deployment with configurable replicas
- Service exposing the application

---

### 2. webapp-with-features.yaml
**Purpose**: Demonstrates conditional resource inclusion using `includeWhen`

**Features**:
- Boolean flags to enable/disable features
- Conditional resources based on schema field values
- CEL expressions in `includeWhen` conditions
- Multiple conditional resources (database, cache, monitoring)

**Schema Fields**:
- `name` (required): Web application name
- `replicas`: Number of replicas (default: 1)
- `enableDatabase`: Enable PostgreSQL database (default: false)
- `enableCache`: Enable Redis cache (default: false)
- `enableMonitoring`: Enable Prometheus monitoring (default: false)

**Conditional Resources**:
- PostgreSQL StatefulSet + Service (when `enableDatabase == true`)
- Redis Deployment + Service (when `enableCache == true`)
- ServiceMonitor (when `enableMonitoring == true`)

**Parser Capabilities Demonstrated**:
- `IncludeWhen` condition parsing
- `SchemaFields` extraction from CEL expressions
- Conditional field visibility in dashboard

---

### 3. microservices-platform.yaml
**Purpose**: Demonstrates external references, complex conditions, and resource dependencies

**Features**:
- External resource references using `externalRef`
- Complex CEL expressions with multiple conditions
- Environment-based configuration
- High availability mode with replica scaling
- Service mesh integration (Istio)

**Schema Fields**:
- `platformName` (required): Platform name
- `environment`: Deployment environment (dev/staging/production)
- `useExistingDatabase`: Use external database (default: false)
- `externalRef.externaldb.name`: External database service name (auto-filled by resource picker)
- `externalRef.externaldb.namespace`: External database service namespace (auto-filled by resource picker)
- `highAvailability`: Enable HA mode (default: false)

**External References**:
- References external database service when `useExistingDatabase == true`
- Uses paired `${schema.spec.externalRef.<id>.name/namespace}` pattern for resource picker auto-fill
- Single dropdown auto-fills both name and namespace from the selected K8s Service

**Parser Capabilities Demonstrated**:
- `ExternalRefInfo` parsing with `NamespaceSchemaField`
- Paired `externalRef.<id>.name/namespace` field detection
- Resource picker metadata on parent object (`AutoFillFields`)
- Complex multi-condition `includeWhen` logic
- Resource dependencies and graph edges

---

## How These RGDs Are Used

When you run `make qa-deploy`, these RGDs are automatically:

1. **Deployed to the Kind cluster** in the `default` namespace
2. **Processed by KRO** to validate the schema and resources
3. **Parsed by the knodex server** to generate resource graphs
4. **Displayed in the dashboard UI** with:
   - Resource nodes and their types
   - Conditional resources with their conditions
   - Schema field dependencies
   - External references

## Testing the Parser

To verify the parser is correctly extracting information:

```bash
# Deploy the dashboard and examples
make qa-deploy

# Query the RGD API
curl http://localhost:8080/api/v1/rgds | jq

# Check specific RGD
curl http://localhost:8080/api/v1/rgds/default/webapp-with-features | jq
```

## Parser Features Covered

| Feature | simple-app | webapp-with-features | microservices-platform |
|---------|-----------|---------------------|------------------------|
| Template resources | ✅ | ✅ | ✅ |
| Schema field interpolation | ✅ | ✅ | ✅ |
| `includeWhen` conditions | ❌ | ✅ | ✅ |
| Multiple conditions | ❌ | ❌ | ✅ |
| External references | ❌ | ❌ | ✅ |
| CEL expressions | ✅ | ✅ | ✅ |
| Schema field extraction | ✅ | ✅ | ✅ |
| Resource dependencies | ❌ | ❌ | ✅ |

## Related Code

The parser implementation that processes these RGDs is located at:
- `server/internal/parser/resource_parser.go` - Main parser logic
- `server/internal/parser/types.go` - Type definitions

The parser extracts:
- Resource definitions (ID, Kind, APIVersion)
- Template vs ExternalRef distinction
- Conditional expressions and schema field dependencies
- Resource dependency graph edges
