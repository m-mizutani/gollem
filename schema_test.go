package gollem_test

import (
	"errors"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
)

func TestToSchemaBasicTypes(t *testing.T) {
	type TestStruct struct {
		Name   string   `json:"name"`
		Age    int      `json:"age"`
		Score  float64  `json:"score"`
		Active bool     `json:"active"`
		Tags   []string `json:"tags"`
	}

	param, err := gollem.ToSchema(TestStruct{})
	gt.NoError(t, err)
	gt.Equal(t, param.Type, gollem.TypeObject)
	gt.Equal(t, len(param.Properties), 5)

	// Check string field
	gt.Equal(t, param.Properties["name"].Type, gollem.TypeString)

	// Check int field
	gt.Equal(t, param.Properties["age"].Type, gollem.TypeInteger)

	// Check float field
	gt.Equal(t, param.Properties["score"].Type, gollem.TypeNumber)

	// Check bool field
	gt.Equal(t, param.Properties["active"].Type, gollem.TypeBoolean)

	// Check array field
	gt.Equal(t, param.Properties["tags"].Type, gollem.TypeArray)
	gt.Equal(t, param.Properties["tags"].Items.Type, gollem.TypeString)
}

func TestToSchemaWithDescription(t *testing.T) {
	type User struct {
		Name string `json:"name" description:"User's full name"`
		Age  int    `json:"age" description:"Age in years"`
	}

	param, err := gollem.ToSchema(User{})
	gt.NoError(t, err)
	gt.Equal(t, param.Properties["name"].Description, "User's full name")
	gt.Equal(t, param.Properties["age"].Description, "Age in years")
}

func TestToSchemaWithEnum(t *testing.T) {
	type User struct {
		Role   string `json:"role" enum:"admin,user,guest"`
		Status string `json:"status" enum:"active, inactive, pending"`
	}

	param, err := gollem.ToSchema(User{})
	gt.NoError(t, err)
	gt.Equal(t, param.Properties["role"].Enum, []string{"admin", "user", "guest"})
	// Enum values should be trimmed
	gt.Equal(t, param.Properties["status"].Enum, []string{"active", "inactive", "pending"})
}

func TestToSchemaWithNumericConstraints(t *testing.T) {
	type Product struct {
		Price    float64 `json:"price" min:"0" max:"10000"`
		Quantity int     `json:"quantity" min:"1" max:"100"`
	}

	param, err := gollem.ToSchema(Product{})
	gt.NoError(t, err)

	// Check price constraints
	gt.NotNil(t, param.Properties["price"].Minimum)
	gt.V(t, *param.Properties["price"].Minimum).Equal(0.0)
	gt.NotNil(t, param.Properties["price"].Maximum)
	gt.V(t, *param.Properties["price"].Maximum).Equal(10000.0)

	// Check quantity constraints
	gt.NotNil(t, param.Properties["quantity"].Minimum)
	gt.V(t, *param.Properties["quantity"].Minimum).Equal(1.0)
	gt.NotNil(t, param.Properties["quantity"].Maximum)
	gt.V(t, *param.Properties["quantity"].Maximum).Equal(100.0)
}

func TestToSchemaWithStringConstraints(t *testing.T) {
	type User struct {
		Username string `json:"username" minLength:"3" maxLength:"20" pattern:"^[a-z0-9]+$"`
		Email    string `json:"email" pattern:"^[a-z@.]+$"`
	}

	param, err := gollem.ToSchema(User{})
	gt.NoError(t, err)

	// Check username constraints
	gt.NotNil(t, param.Properties["username"].MinLength)
	gt.V(t, *param.Properties["username"].MinLength).Equal(3)
	gt.NotNil(t, param.Properties["username"].MaxLength)
	gt.V(t, *param.Properties["username"].MaxLength).Equal(20)
	gt.Equal(t, param.Properties["username"].Pattern, "^[a-z0-9]+$")

	// Check email pattern
	gt.Equal(t, param.Properties["email"].Pattern, "^[a-z@.]+$")
}

func TestToSchemaWithArrayConstraints(t *testing.T) {
	type Post struct {
		Tags []string `json:"tags" minItems:"1" maxItems:"10"`
	}

	param, err := gollem.ToSchema(Post{})
	gt.NoError(t, err)

	gt.NotNil(t, param.Properties["tags"].MinItems)
	gt.V(t, *param.Properties["tags"].MinItems).Equal(1)
	gt.NotNil(t, param.Properties["tags"].MaxItems)
	gt.V(t, *param.Properties["tags"].MaxItems).Equal(10)
}

func TestToSchemaWithRequired(t *testing.T) {
	type User struct {
		Name  string `json:"name" required:"true"`
		Email string `json:"email" required:"true"`
		Age   int    `json:"age"`
	}

	param, err := gollem.ToSchema(User{})
	gt.NoError(t, err)
	gt.A(t, param.Required).Length(2)

	// Check required fields
	hasName := false
	hasEmail := false
	for _, req := range param.Required {
		if req == "name" {
			hasName = true
		}
		if req == "email" {
			hasEmail = true
		}
	}
	gt.True(t, hasName)
	gt.True(t, hasEmail)
}

func TestToSchemaNestedStruct(t *testing.T) {
	type Address struct {
		Street  string `json:"street" required:"true"`
		City    string `json:"city" required:"true"`
		Country string `json:"country" required:"true"`
	}

	type User struct {
		Name    string  `json:"name" required:"true"`
		Address Address `json:"address" required:"true"`
	}

	param, err := gollem.ToSchema(User{})
	gt.NoError(t, err)
	gt.Equal(t, param.Type, gollem.TypeObject)
	gt.Equal(t, len(param.Required), 2)

	// Check nested struct
	address := param.Properties["address"]
	gt.Equal(t, address.Type, gollem.TypeObject)
	gt.Equal(t, len(address.Properties), 3)
	gt.A(t, address.Required).Length(3)

	// Check all required fields are present
	hasStreet, hasCity, hasCountry := false, false, false
	for _, req := range address.Required {
		switch req {
		case "street":
			hasStreet = true
		case "city":
			hasCity = true
		case "country":
			hasCountry = true
		}
	}
	gt.True(t, hasStreet && hasCity && hasCountry)
}

func TestToSchemaArrayOfStructs(t *testing.T) {
	type Item struct {
		Name  string  `json:"name" required:"true"`
		Price float64 `json:"price" min:"0"`
	}

	type Order struct {
		Items []Item `json:"items" minItems:"1"`
	}

	param, err := gollem.ToSchema(Order{})
	gt.NoError(t, err)

	items := param.Properties["items"]
	gt.Equal(t, items.Type, gollem.TypeArray)
	gt.NotNil(t, items.MinItems)
	gt.V(t, *items.MinItems).Equal(1)

	// Check array element type
	gt.Equal(t, items.Items.Type, gollem.TypeObject)
	gt.Equal(t, len(items.Items.Properties), 2)
	gt.A(t, items.Items.Required).Length(1)
	gt.Equal(t, items.Items.Required[0], "name")
}

func TestToSchemaIgnoredFields(t *testing.T) {
	type User struct {
		Name     string `json:"name"`
		Password string `json:"-"` // Should be ignored
	}

	param, err := gollem.ToSchema(User{})
	gt.NoError(t, err)
	gt.Equal(t, len(param.Properties), 1) // Only "name" should be included
	gt.NotNil(t, param.Properties["name"])
	gt.Nil(t, param.Properties["Password"])
}

func TestToSchemaPointerType(t *testing.T) {
	type User struct {
		Name *string `json:"name"`
	}

	param, err := gollem.ToSchema(User{})
	gt.NoError(t, err)
	gt.Equal(t, param.Properties["name"].Type, gollem.TypeString)
}

func TestToSchemaComplexExample(t *testing.T) {
	type SecurityAlert struct {
		Severity        string   `json:"severity" enum:"low,medium,high,critical" required:"true"`
		ThreatType      string   `json:"threat_type" required:"true"`
		AffectedSystems []string `json:"affected_systems" minItems:"1"`
		IPAddresses     []string `json:"ip_addresses"`
		Confidence      float64  `json:"confidence" min:"0" max:"1"`
	}

	param, err := gollem.ToSchema(SecurityAlert{})
	gt.NoError(t, err)
	gt.Equal(t, param.Type, gollem.TypeObject)
	gt.Equal(t, len(param.Required), 2)

	// Check enum field
	severity := param.Properties["severity"]
	gt.Equal(t, severity.Type, gollem.TypeString)
	gt.Equal(t, len(severity.Enum), 4)

	// Check array with constraints
	affected := param.Properties["affected_systems"]
	gt.Equal(t, affected.Type, gollem.TypeArray)
	gt.NotNil(t, affected.MinItems)

	// Check numeric constraints
	confidence := param.Properties["confidence"]
	gt.NotNil(t, confidence.Minimum)
	gt.V(t, *confidence.Minimum).Equal(0.0)
	gt.NotNil(t, confidence.Maximum)
	gt.V(t, *confidence.Maximum).Equal(1.0)
}

func TestToSchemaInvalidTags(t *testing.T) {
	type InvalidMin struct {
		Value int `json:"value" min:"invalid"`
	}

	_, err := gollem.ToSchema(InvalidMin{})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, gollem.ErrInvalidTag))
}

func TestToSchemaCyclicReference(t *testing.T) {
	type Node struct {
		Value string `json:"value"`
		Next  *Node  `json:"next"`
	}

	_, err := gollem.ToSchema(Node{})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, gollem.ErrCyclicReference))
}

func TestMustToSchemaPanic(t *testing.T) {
	type Invalid struct {
		Chan chan int `json:"chan"` // Unsupported type
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustToSchema should panic on unsupported type")
		}
	}()

	gollem.MustToSchema(Invalid{})
}

func TestToSchemaWithTitleAndDescription(t *testing.T) {
	type UserProfile struct {
		Name  string `json:"name" required:"true"`
		Email string `json:"email" required:"true"`
	}

	schema, err := gollem.ToSchema(UserProfile{})
	gt.NoError(t, err)

	// Set Title and Description
	schema.Title = "UserProfile"
	schema.Description = "Structured user profile information"

	gt.Equal(t, schema.Title, "UserProfile")
	gt.Equal(t, schema.Description, "Structured user profile information")
	gt.Equal(t, schema.Type, gollem.TypeObject)
	gt.Equal(t, len(schema.Required), 2)
}

func TestToSchemaMapType(t *testing.T) {
	type Config struct {
		Settings map[string]string `json:"settings"`
	}

	param, err := gollem.ToSchema(Config{})
	gt.NoError(t, err)
	gt.Equal(t, param.Properties["settings"].Type, gollem.TypeObject)
}

func TestToSchemaAllIntegerTypes(t *testing.T) {
	type Numbers struct {
		I8   int8   `json:"i8"`
		I16  int16  `json:"i16"`
		I32  int32  `json:"i32"`
		I64  int64  `json:"i64"`
		U    uint   `json:"u"`
		U8   uint8  `json:"u8"`
		U16  uint16 `json:"u16"`
		U32  uint32 `json:"u32"`
		U64  uint64 `json:"u64"`
		F32  float32 `json:"f32"`
	}

	param, err := gollem.ToSchema(Numbers{})
	gt.NoError(t, err)

	// All int types should be TypeInteger
	gt.Equal(t, param.Properties["i8"].Type, gollem.TypeInteger)
	gt.Equal(t, param.Properties["i16"].Type, gollem.TypeInteger)
	gt.Equal(t, param.Properties["i32"].Type, gollem.TypeInteger)
	gt.Equal(t, param.Properties["i64"].Type, gollem.TypeInteger)
	gt.Equal(t, param.Properties["u"].Type, gollem.TypeInteger)
	gt.Equal(t, param.Properties["u8"].Type, gollem.TypeInteger)
	gt.Equal(t, param.Properties["u16"].Type, gollem.TypeInteger)
	gt.Equal(t, param.Properties["u32"].Type, gollem.TypeInteger)
	gt.Equal(t, param.Properties["u64"].Type, gollem.TypeInteger)

	// float32 should be TypeNumber
	gt.Equal(t, param.Properties["f32"].Type, gollem.TypeNumber)
}

func TestToSchemaArrayType(t *testing.T) {
	type FixedArray struct {
		Items [5]string `json:"items"`
	}

	param, err := gollem.ToSchema(FixedArray{})
	gt.NoError(t, err)
	gt.Equal(t, param.Properties["items"].Type, gollem.TypeArray)
	gt.Equal(t, param.Properties["items"].Items.Type, gollem.TypeString)
}

func TestToSchemaUnexportedFields(t *testing.T) {
	type User struct {
		Name     string `json:"name"`
		password string //nolint:unused // intentionally unexported for testing
	}

	param, err := gollem.ToSchema(User{})
	gt.NoError(t, err)
	gt.Equal(t, len(param.Properties), 1) // Only exported field
	gt.NotNil(t, param.Properties["name"])
	gt.Nil(t, param.Properties["password"])
}

func TestToSchemaNilInput(t *testing.T) {
	_, err := gollem.ToSchema(nil)
	gt.Error(t, err)
	gt.True(t, errors.Is(err, gollem.ErrUnsupportedType))
}

func TestToSchemaInvalidRequiredTag(t *testing.T) {
	type User struct {
		Name string `json:"name" required:"invalid"`
	}

	_, err := gollem.ToSchema(User{})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, gollem.ErrInvalidTag))
}

func TestToSchemaInvalidMaxTag(t *testing.T) {
	type Product struct {
		Price float64 `json:"price" max:"not_a_number"`
	}

	_, err := gollem.ToSchema(Product{})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, gollem.ErrInvalidTag))
}

func TestToSchemaInvalidMinLengthTag(t *testing.T) {
	type User struct {
		Name string `json:"name" minLength:"abc"`
	}

	_, err := gollem.ToSchema(User{})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, gollem.ErrInvalidTag))
}

func TestToSchemaInvalidMaxItemsTag(t *testing.T) {
	type Post struct {
		Tags []string `json:"tags" maxItems:"xyz"`
	}

	_, err := gollem.ToSchema(Post{})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, gollem.ErrInvalidTag))
}

func TestToSchemaUnsupportedType(t *testing.T) {
	type Invalid struct {
		Ch chan int `json:"ch"`
	}

	_, err := gollem.ToSchema(Invalid{})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, gollem.ErrUnsupportedType))
}
