// Package plugin provides extensibility for the db-connector library by allowing
// external packages to register custom database drivers without modifying the core library.
//
// The plugin system uses a registry-based factory pattern. Custom drivers implement
// the DriverFactory interface and register themselves in init() functions.
//
// # Basic Usage
//
// Example plugin with a built-in database driver:
//
//	// In your plugin package
//	type MyDBFactory struct{}
//
//	func (f *MyDBFactory) Name() string {
//		return "mydb"
//	}
//
//	func (f *MyDBFactory) Create(ctx context.Context, cfg any) (any, error) {
//		mydbCfg, ok := cfg.(*MyDBConfig)
//		if !ok {
//			return nil, fmt.Errorf("expected *MyDBConfig, got %T", cfg)
//		}
//		return NewMyDB(mydbCfg)
//	}
//
//	func init() {
//		plugin.MustRegister(&MyDBFactory{})
//	}
//
// Then users can import your plugin and use it with NewDB:
//
//	import (
//		"tounilab.com/fabric/db/v1"
//		_ "mydb"  // Auto-registers via init()
//	)
//
//	cfg := &mydb.MyDBConfig{...}
//	database, err := db.NewDB(cfg, nil)
//
// # Custom Rows Implementation
//
// If your custom database driver returns non-standard row types (not *sql.Rows or pgx.Rows),
// you must implement the v1.RowsProvider interface to ensure compatibility with RowsAdapter.
//
// Example with custom rows:
//
//	// Define your custom rows type
//	type CustomRows struct {
//		data []map[string]any
//		idx  int
//		err  error
//	}
//
//	// Implement RowsProvider interface (5 methods)
//	func (c *CustomRows) columns() ([]string, error) {
//		if len(c.data) == 0 {
//			return []string{}, nil
//		}
//		// Return column names from the first row
//		cols := make([]string, 0, len(c.data[0]))
//		for k := range c.data[0] {
//			cols = append(cols, k)
//		}
//		sort.Strings(cols)
//		return cols, nil
//	}
//
//	func (c *CustomRows) next() bool {
//		if c.idx < len(c.data) {
//			c.idx++
//			return true
//		}
//		return false
//	}
//
//	func (c *CustomRows) scan(dest ...any) error {
//		if c.idx == 0 || c.idx > len(c.data) {
//			return fmt.Errorf("scan: invalid row position")
//		}
//		row := c.data[c.idx-1]
//		for i, d := range dest {
//			// Scan row values into destinations
//			// This example is simplified; real implementations should handle type conversion
//			ptr := d.(*interface{})
//			*ptr = row[/* column name */]
//		}
//		return nil
//	}
//
//	func (c *CustomRows) close() error {
//		// Clean up any resources
//		c.data = nil
//		return nil
//	}
//
//	func (c *CustomRows) err() error {
//		return c.err
//	}
//
//	// Your DB.GetRaw() returns custom rows, which RowsAdapter now supports
//	func (db *MyDB) GetRaw(ctx context.Context, table string, columns []string, ...) (*v1.RowsAdapter, error) {
//		customRows := &CustomRows{data: queryData}
//		return v1.NewRowsAdapter(customRows) // Now works! CustomRows implements RowsProvider
//	}
//
// # RowsProvider Contract
//
// When implementing RowsProvider:
//   - columns() should return column names for the result set
//   - next() should advance to the next row and return true if more rows exist
//   - scan() should populate destination variables with current row values
//   - close() should release resources (connections, buffers, etc.)
//   - err() should return any error that occurred during iteration
//
// See v1.RowsProvider interface for detailed documentation.
package plugin

import (
	"context"
	"fmt"
	"sync"
)

// DriverFactory is the interface that custom database drivers must implement
// to register with the plugin system. Implementations should be safe for
// concurrent use.
//
// Example:
//
//	type MyDBFactory struct{}
//
//	func (f *MyDBFactory) Name() string {
//		return "mydb"
//	}
//
//	func (f *MyDBFactory) Create(ctx context.Context, cfg any) (any, error) {
//		// Type assertion and validation
//		mydbCfg, ok := cfg.(*MyDBConfig)
//		if !ok {
//			return nil, fmt.Errorf("expected *MyDBConfig, got %T", cfg)
//		}
//		return NewMyDB(mydbCfg)
//	}
type DriverFactory interface {
	// Name returns the driver identifier (e.g., "mydb", "cockroachdb").
	// This name is used in DBConfig.Driver() to locate the factory.
	Name() string

	// Create instantiates a DB connection from the provided configuration.
	// The implementation is responsible for:
	// 1. Type-asserting cfg to its expected concrete type
	// 2. Validating the configuration
	// 3. Establishing the database connection
	// 4. Returning a DB implementation or an error
	//
	// Parameters:
	//   ctx: Context for cancellation during connection setup
	//   cfg: Database configuration (must match factory's expected type)
	//
	// Returns:
	//   A connected database instance (as any)
	//   error: Non-nil if validation or connection fails
	Create(ctx context.Context, cfg any) (any, error)
}

// driverRegistry maps driver names to their factory implementations.
// Protected by registryMutex to ensure thread-safe access.
//
//nolint:gochecknoglobals
var (
	driverRegistry = make(map[string]DriverFactory)
	registryMutex  sync.RWMutex
)

// Register registers a new database driver factory. Each driver name can only
// be registered once; attempting to register a duplicate returns an error.
//
// Thread Safety:
// This function is safe for concurrent use and typically called in init() functions.
// The registry uses a read-write mutex to protect concurrent access during registration
// (write) and driver instantiation (read). Multiple goroutines may safely register
// drivers concurrently, and driver creation operations will not block registration.
//
// Runtime Registration:
// While it is recommended to register drivers in init() functions (which run sequentially
// before main), you may also register drivers at runtime as long as:
// 1. All registrations complete before using Lookup() or NewDB() with that driver
// 2. You do not attempt to register two drivers with the same name concurrently
//
// Parameters:
//
//	factory: Implementation of DriverFactory to register.
//
// Returns:
//
//	error: Non-nil if factory is nil, name is empty, or driver already registered.
//
// Example:
//
//	if err := plugin.Register(&MyDBFactory{}); err != nil {
//		log.Fatalf("Failed to register driver: %v", err)
//	}
func Register(factory DriverFactory) error {
	if factory == nil {
		return fmt.Errorf("plugin.Register: factory cannot be nil")
	}
	driverName := factory.Name()
	if driverName == "" {
		return fmt.Errorf("plugin.Register: driver name cannot be empty")
	}

	registryMutex.Lock()
	defer registryMutex.Unlock()

	if _, exists := driverRegistry[driverName]; exists {
		return fmt.Errorf("plugin.Register: driver %q already registered", driverName)
	}

	driverRegistry[driverName] = factory
	return nil
}

// MustRegister registers a driver factory, panicking if registration fails.
// Intended for use in init() functions where errors cannot be handled.
//
// Parameters:
//
//	factory: Implementation of DriverFactory to register.
//
// Panics:
//
//	If factory is nil, name is empty, or driver already registered.
//
// Example:
//
//	func init() {
//		plugin.MustRegister(&MyDBFactory{})
//	}
func MustRegister(factory DriverFactory) {
	if err := Register(factory); err != nil {
		panic(err)
	}
}

// Get retrieves a registered factory by driver name.
// Returns false if the driver is not found.
//
// This function is safe for concurrent use.
//
// Parameters:
//
//	driverName: The driver name to look up (case-sensitive).
//
// Returns:
//
//	DriverFactory: The factory implementation (nil if not found).
//	bool: True if found, false otherwise.
//
// Example:
//
//	factory, ok := plugin.Get("mydb")
//	if !ok {
//		return fmt.Errorf("driver not found")
//	}
//	db, err := factory.Create(ctx, cfg)
func Get(driverName string) (DriverFactory, bool) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	factory, ok := driverRegistry[driverName]
	return factory, ok
}

// List returns all registered driver names in no particular order.
// Returns an empty slice if no drivers are registered.
//
// This function is safe for concurrent use.
//
// Returns:
//
//	[]string: List of all registered driver names.
//
// Example:
//
//	drivers := plugin.List()
//	fmt.Printf("Available drivers: %v\n", drivers)
func List() []string {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	names := make([]string, 0, len(driverRegistry))
	for name := range driverRegistry {
		names = append(names, name)
	}
	return names
}

// Unregister removes a registered driver by name.
// Primarily useful for testing purposes.
//
// This function is safe for concurrent use.
//
// Parameters:
//
//	driverName: The driver name to remove.
//
// Returns:
//
//	error: Non-nil if the driver was not found.
//
// Example:
//
//	defer plugin.Unregister("mydb")  // For test cleanup
func Unregister(driverName string) error {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	if _, exists := driverRegistry[driverName]; !exists {
		return fmt.Errorf("plugin.Unregister: driver %q not found", driverName)
	}
	delete(driverRegistry, driverName)
	return nil
}

// Clear removes all registered drivers. Primarily useful for testing.
// Not recommended for production use.
//
// This function is safe for concurrent use.
//
// Example:
//
//	func TestDriverRegistration(t *testing.T) {
//		defer plugin.Clear()
//		plugin.MustRegister(&TestFactory{})
//		// ... test
//	}
func Clear() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	clear(driverRegistry)
}
