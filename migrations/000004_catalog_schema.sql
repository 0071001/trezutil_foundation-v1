-- +goose Up
-- SQL in section 'Up' is executed when migration is applied

-- Create categories table
CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    icon_url TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    scope VARCHAR(50) NOT NULL DEFAULT 'global',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(slug, scope, parent_id),
    CHECK (status IN ('active', 'inactive', 'system')),
    CHECK (scope IN ('global', 'system', 'tenant'))
);

-- Create attribute_definitions table
CREATE TABLE IF NOT EXISTS attribute_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key VARCHAR(100) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    unit VARCHAR(50),
    required BOOLEAN NOT NULL DEFAULT false,
    searchable BOOLEAN NOT NULL DEFAULT false,
    filterable BOOLEAN NOT NULL DEFAULT false,
    options TEXT,
    validation TEXT,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(category_id, key),
    CHECK (type IN ('text', 'number', 'boolean', 'date', 'url', 'select', 'multiselect'))
);

-- Create listings table
CREATE TABLE IF NOT EXISTS listings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    seller_id UUID NOT NULL REFERENCES seller_profiles(id) ON DELETE CASCADE,
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    listing_type VARCHAR(50) NOT NULL,
    pricing_model VARCHAR(50) NOT NULL,
    price_amount DECIMAL(12, 2) NOT NULL,
    price_currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    metadata JSONB,
    published_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (listing_type IN ('physical_product', 'real_estate', 'vehicle', 'food', 'service', 'other')),
    CHECK (pricing_model IN ('fixed', 'negotiable', 'auction')),
    CHECK (status IN ('draft', 'pending_review', 'published', 'paused', 'sold', 'closed', 'expired', 'suspended')),
    CHECK (price_amount >= 0)
);

-- Create listing_attributes table
CREATE TABLE IF NOT EXISTS listing_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    attribute_definition_id UUID NOT NULL REFERENCES attribute_definitions(id) ON DELETE CASCADE,
    value_text TEXT,
    value_number DECIMAL(12, 4),
    value_boolean BOOLEAN,
    value_date TIMESTAMP WITH TIME ZONE,
    value_url TEXT,
    value_select VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(listing_id, attribute_definition_id)
);

-- Create listing_media table
CREATE TABLE IF NOT EXISTS listing_media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    url TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (type IN ('image', 'video', 'document'))
);

-- Create listing_availability table
CREATE TABLE IF NOT EXISTS listing_availability (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'in_stock',
    quantity INTEGER,
    reserved_quantity INTEGER DEFAULT 0,
    available_from TIMESTAMP WITH TIME ZONE,
    available_to TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(listing_id),
    CHECK (status IN ('in_stock', 'out_of_stock', 'pre_order', 'reserved', 'available', 'unavailable')),
    CHECK (quantity IS NULL OR quantity >= 0),
    CHECK (reserved_quantity IS NULL OR reserved_quantity >= 0)
);

-- Create locations table
CREATE TABLE IF NOT EXISTS locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID REFERENCES locations(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    country VARCHAR(2) NOT NULL,
    province VARCHAR(100),
    city VARCHAR(100),
    neighborhood VARCHAR(100),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    timezone VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (type IN ('country', 'province', 'city', 'neighborhood')),
    CHECK (length(country) = 2),
    CHECK (latitude IS NULL OR (latitude >= -90 AND latitude <= 90)),
    CHECK (longitude IS NULL OR (longitude >= -180 AND longitude <= 180))
);

-- Create listing_locations table
CREATE TABLE IF NOT EXISTS listing_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    label VARCHAR(255),
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(listing_id, location_id)
);

-- Create user_locations table
CREATE TABLE IF NOT EXISTS user_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    label VARCHAR(255),
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, location_id)
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_categories_parent_id ON categories(parent_id);
CREATE INDEX IF NOT EXISTS idx_categories_slug ON categories(slug);
CREATE INDEX IF NOT EXISTS idx_categories_status ON categories(status);
CREATE INDEX IF NOT EXISTS idx_categories_scope ON categories(scope);

CREATE INDEX IF NOT EXISTS idx_attribute_definitions_category_id ON attribute_definitions(category_id);
CREATE INDEX IF NOT EXISTS idx_attribute_definitions_key ON attribute_definitions(key);

CREATE INDEX IF NOT EXISTS idx_listings_organization_id ON listings(organization_id);
CREATE INDEX IF NOT EXISTS idx_listings_seller_id ON listings(seller_id);
CREATE INDEX IF NOT EXISTS idx_listings_store_id ON listings(store_id);
CREATE INDEX IF NOT EXISTS idx_listings_category_id ON listings(category_id);
CREATE INDEX IF NOT EXISTS idx_listings_status ON listings(status);
CREATE INDEX IF NOT EXISTS idx_listings_price_amount ON listings(price_amount);
CREATE INDEX IF NOT EXISTS idx_listings_published_at ON listings(published_at);
CREATE INDEX IF NOT EXISTS idx_listings_created_at ON listings(created_at);

CREATE INDEX IF NOT EXISTS idx_listing_attributes_listing_id ON listing_attributes(listing_id);
CREATE INDEX IF NOT EXISTS idx_listing_attributes_definition_id ON listing_attributes(attribute_definition_id);

CREATE INDEX IF NOT EXISTS idx_listing_media_listing_id ON listing_media(listing_id);

CREATE INDEX IF NOT EXISTS idx_listing_availability_listing_id ON listing_availability(listing_id);

CREATE INDEX IF NOT EXISTS idx_locations_country ON locations(country);
CREATE INDEX IF NOT EXISTS idx_locations_type ON locations(type);
CREATE INDEX IF NOT EXISTS idx_locations_parent_id ON locations(parent_id);

CREATE INDEX IF NOT EXISTS idx_listing_locations_listing_id ON listing_locations(listing_id);
CREATE INDEX IF NOT EXISTS idx_listing_locations_location_id ON listing_locations(location_id);

CREATE INDEX IF NOT EXISTS idx_user_locations_user_id ON user_locations(user_id);
CREATE INDEX IF NOT EXISTS idx_user_locations_location_id ON user_locations(location_id);

-- +goose Down
-- SQL in section 'Down' is executed when migration is rolled back

DROP INDEX IF EXISTS idx_user_locations_location_id;
DROP INDEX IF EXISTS idx_user_locations_user_id;
DROP INDEX IF EXISTS idx_listing_locations_location_id;
DROP INDEX IF EXISTS idx_listing_locations_listing_id;
DROP INDEX IF EXISTS idx_locations_parent_id;
DROP INDEX IF EXISTS idx_locations_type;
DROP INDEX IF EXISTS idx_locations_country;
DROP INDEX IF EXISTS idx_listing_availability_listing_id;
DROP INDEX IF EXISTS idx_listing_media_listing_id;
DROP INDEX IF EXISTS idx_listing_attributes_definition_id;
DROP INDEX IF EXISTS idx_listing_attributes_listing_id;
DROP INDEX IF EXISTS idx_listings_created_at;
DROP INDEX IF EXISTS idx_listings_published_at;
DROP INDEX IF EXISTS idx_listings_price_amount;
DROP INDEX IF EXISTS idx_listings_status;
DROP INDEX IF EXISTS idx_listings_category_id;
DROP INDEX IF EXISTS idx_listings_store_id;
DROP INDEX IF EXISTS idx_listings_seller_id;
DROP INDEX IF EXISTS idx_listings_organization_id;
DROP INDEX IF EXISTS idx_attribute_definitions_key;
DROP INDEX IF EXISTS idx_attribute_definitions_category_id;
DROP INDEX IF EXISTS idx_categories_scope;
DROP INDEX IF EXISTS idx_categories_status;
DROP INDEX IF EXISTS idx_categories_slug;
DROP INDEX IF EXISTS idx_categories_parent_id;

DROP TABLE IF EXISTS user_locations;
DROP TABLE IF EXISTS listing_locations;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS listing_availability;
DROP TABLE IF EXISTS listing_media;
DROP TABLE IF EXISTS listing_attributes;
DROP TABLE IF EXISTS listings;
DROP TABLE IF EXISTS attribute_definitions;
DROP TABLE IF EXISTS categories;
