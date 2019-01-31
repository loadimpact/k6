/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

import (
	null "gopkg.in/guregu/null.v3"
)

// ExtraConfig contains extra statsd config
type ExtraConfig struct {
	Namespace    string
	TagWhitelist string
}

// Config defines the statsd configuration
type Config struct {
	Addr         null.String `json:"addr,omitempty" default:"localhost:8125"`
	BufferSize   null.Int    `json:"buffer_size,omitempty" default:"20"`
	Namespace    null.String `json:"namespace,omitempty"`
	TagWhitelist null.String `json:"tag_whitelist,omitempty" default:"status, method"`
}

// Apply returns config with defaults applied
func (c Config) Apply(cfg Config) Config {
	if cfg.Addr.Valid {
		c.Addr = cfg.Addr
	}
	if cfg.BufferSize.Valid {
		c.BufferSize = cfg.BufferSize
	}
	if cfg.Namespace.Valid {
		c.Namespace = cfg.Namespace
	}
	if cfg.TagWhitelist.Valid {
		c.TagWhitelist = cfg.TagWhitelist
	}

	return c
}
