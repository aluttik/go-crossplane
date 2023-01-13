package crossplane

import "fmt"

// bit masks for different directive argument styles
const (
	ngxConfNoArgs = 0x00000001 // 0 args
	ngxConfTake1  = 0x00000002 // 1 args
	ngxConfTake2  = 0x00000004 // 2 args
	ngxConfTake3  = 0x00000008 // 3 args
	ngxConfTake4  = 0x00000010 // 4 args
	ngxConfTake5  = 0x00000020 // 5 args
	ngxConfTake6  = 0x00000040 // 6 args
	// ngxConfTake7  = 0x00000080 // 7 args (currently unused)
	ngxConfBlock = 0x00000100 // followed by block
	ngxConfFlag  = 0x00000200 // 'on' or 'off'
	ngxConfAny   = 0x00000400 // >=0 args
	ngxConf1More = 0x00000800 // >=1 args
	ngxConf2More = 0x00001000 // >=2 args

	// some helpful argument style aliases
	ngxConfTake12 = (ngxConfTake1 | ngxConfTake2)
	//ngxConfTake13   = (ngxConfTake1 | ngxConfTake3) (currently unused)
	ngxConfTake23   = (ngxConfTake2 | ngxConfTake3)
	ngxConfTake34   = (ngxConfTake3 | ngxConfTake4)
	ngxConfTake123  = (ngxConfTake12 | ngxConfTake3)
	ngxConfTake1234 = (ngxConfTake123 | ngxConfTake4)

	// bit masks for different directive locations
	ngxDirectConf     = 0x00010000 // main file (not used)
	ngxMainConf       = 0x00040000 // main context
	ngxEventConf      = 0x00080000 // events
	ngxMailMainConf   = 0x00100000 // mail
	ngxMailSrvConf    = 0x00200000 // mail > server
	ngxStreamMainConf = 0x00400000 // stream
	ngxStreamSrvConf  = 0x00800000 // stream > server
	ngxStreamUpsConf  = 0x01000000 // stream > upstream
	ngxHttpMainConf   = 0x02000000 // http
	ngxHttpSrvConf    = 0x04000000 // http > server
	ngxHttpLocConf    = 0x08000000 // http > location
	ngxHttpUpsConf    = 0x10000000 // http > upstream
	ngxHttpSifConf    = 0x20000000 // http > server > if
	ngxHttpLifConf    = 0x40000000 // http > location > if
	ngxHttpLmtConf    = 0x80000000 // http > location > limit_except
)

// helpful directive location alias describing "any" context
// doesn't include ngxHttpSifConf, ngxHttpLifConf, or ngxHttpLmtConf
const ngxAnyConf = (ngxMainConf | ngxEventConf | ngxMailMainConf | ngxMailSrvConf |
	ngxStreamMainConf | ngxStreamSrvConf | ngxStreamUpsConf |
	ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpUpsConf)

// map for getting bitmasks from certain context tuples
var contexts = map[string]int{
	blockCtx{}.key():                                   ngxMainConf,
	blockCtx{"events"}.key():                           ngxEventConf,
	blockCtx{"mail"}.key():                             ngxMailMainConf,
	blockCtx{"mail", "server"}.key():                   ngxMailSrvConf,
	blockCtx{"stream"}.key():                           ngxStreamMainConf,
	blockCtx{"stream", "server"}.key():                 ngxStreamSrvConf,
	blockCtx{"stream", "upstream"}.key():               ngxStreamUpsConf,
	blockCtx{"http"}.key():                             ngxHttpMainConf,
	blockCtx{"http", "server"}.key():                   ngxHttpSrvConf,
	blockCtx{"http", "location"}.key():                 ngxHttpLocConf,
	blockCtx{"http", "upstream"}.key():                 ngxHttpUpsConf,
	blockCtx{"http", "server", "if"}.key():             ngxHttpSifConf,
	blockCtx{"http", "location", "if"}.key():           ngxHttpLifConf,
	blockCtx{"http", "location", "limit_except"}.key(): ngxHttpLmtConf,
}

func enterBlockCtx(stmt Directive, ctx blockCtx) blockCtx {
	// don't nest because ngxHttpLocConf just means "location block in http"
	if len(ctx) > 0 && ctx[0] == "http" && stmt.Directive == "location" {
		return blockCtx{"http", "location"}
	}
	// no other block contexts can be nested like location so just append it
	return append(ctx, stmt.Directive)
}

func analyze(fname string, stmt Directive, term string, ctx blockCtx, options *ParseOptions) error {
	masks, knownDirective := directives[stmt.Directive]
	currCtx, knownContext := contexts[ctx.key()]

	// if strict and directive isn't recognized then throw error
	if options.ErrorOnUnknownDirectives && !knownDirective {
		return ParseError{
			what: fmt.Sprintf(`unknown directive "%s"`, stmt.Directive),
			file: &fname,
			line: &stmt.Line,
		}
	}

	// if we don't know where this directive is allowed and how
	// many arguments it can take then don't bother analyzing it
	if !knownContext || !knownDirective {
		return nil
	}

	// if this directive can't be used in this context then throw an error
	var ctxMasks []int
	if options.SkipDirectiveContextCheck {
		ctxMasks = masks
	} else {
		for _, mask := range masks {
			if (mask & currCtx) != 0 {
				ctxMasks = append(ctxMasks, mask)
			}
		}
		if len(ctxMasks) == 0 {
			return ParseError{
				what: fmt.Sprintf(`"%s" directive is not allowed here`, stmt.Directive),
				file: &fname,
				line: &stmt.Line,
			}
		}
	}

	if options.SkipDirectiveArgsCheck {
		return nil
	}

	// do this in reverse because we only throw errors at the end if no masks
	// are valid, and typically the first bit mask is what the parser expects
	var what string
	for i := 0; i < len(ctxMasks); i++ {
		mask := ctxMasks[i]

		// if the directive isn't a block but should be according to the mask
		if (mask&ngxConfBlock) != 0 && term != "{" {
			what = fmt.Sprintf(`directive "%s" has no opening "{"`, stmt.Directive)
			continue
		}

		// if the directive is a block but shouldn't be according to the mask
		if (mask&ngxConfBlock) == 0 && term != ";" {
			what = fmt.Sprintf(`directive "%s" is not terminated by ";"`, stmt.Directive)
			continue
		}

		// use mask to check the directive's arguments
		if ((mask>>len(stmt.Args)&1) != 0 && len(stmt.Args) <= 7) || // NOARGS to TAKE7
			((mask&ngxConfFlag) != 0 && len(stmt.Args) == 1 && validFlag(stmt.Args[0])) ||
			((mask&ngxConfAny) != 0 && len(stmt.Args) >= 0) ||
			((mask&ngxConf1More) != 0 && len(stmt.Args) >= 1) ||
			((mask&ngxConf2More) != 0 && len(stmt.Args) >= 2) {
			return nil
		} else if (mask&ngxConfFlag) != 0 && len(stmt.Args) == 1 && !validFlag(stmt.Args[0]) {
			what = fmt.Sprintf(`invalid value "%s" in "%s" directive, it must be "on" or "off"`, stmt.Args[0], stmt.Directive)
		} else {
			what = fmt.Sprintf(`invalid number of arguments in "%s" directive. found %d`, stmt.Directive, len(stmt.Args))
		}
	}

	return ParseError{
		what: what,
		file: &fname,
		line: &stmt.Line,
	}
}

// This dict maps directives to lists of bit masks that define their behavior.
//
// Each bit mask describes these behaviors:
//   - how many arguments the directive can take
//   - whether or not it is a block directive
//   - whether this is a flag (takes one argument that's either "on" or "off")
//   - which contexts it's allowed to be in
//
// Since some directives can have different behaviors in different contexts, we
//
//	use lists of bit masks, each describing a valid way to use the directive.
//
// Definitions for directives that're available in the open source version of
//
//	nginx were taken directively from the source code. In fact, the variable
//	names for the bit masks defined above were taken from the nginx source code.
//
// Definitions for directives that're only available for nginx+ were inferred
//
//	from the documentation at http://nginx.org/en/docs/.
var directives = map[string][]int{
	"absolute_redirect": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"accept_mutex": []int{
		ngxEventConf | ngxConfFlag,
	},
	"accept_mutex_delay": []int{
		ngxEventConf | ngxConfTake1,
	},
	"access_log": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxHttpLmtConf | ngxConf1More,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConf1More,
	},
	"add_after_body": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"add_before_body": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"add_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake23,
	},
	"add_trailer": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake23,
	},
	"addition_types": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"aio": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"aio_write": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"alias": []int{
		ngxHttpLocConf | ngxConfTake1,
	},
	"allow": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLmtConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ancient_browser": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"ancient_browser_value": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"auth_basic": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLmtConf | ngxConfTake1,
	},
	"auth_basic_user_file": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLmtConf | ngxConfTake1,
	},
	"auth_http": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
	},
	"auth_Httpheader": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake2,
	},
	"auth_Httppass_client_cert": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfFlag,
	},
	"auth_Httptimeout": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
	},
	"auth_request": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"auth_request_set": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"autoindex": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"autoindex_exact_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"autoindex_format": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"autoindex_localtime": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"break": []int{
		ngxHttpSrvConf | ngxHttpSifConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfNoArgs,
	},
	"charset": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"charset_map": []int{
		ngxHttpMainConf | ngxConfBlock | ngxConfTake2,
	},
	"charset_types": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"chunked_transfer_encoding": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"client_body_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"client_body_in_file_only": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"client_body_in_single_buffer": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"client_body_temp_path": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1234,
	},
	"client_body_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"client_header_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"client_header_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"client_max_body_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"connection_pool_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"create_full_put_path": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"daemon": []int{
		ngxMainConf | ngxDirectConf | ngxConfFlag,
	},
	"dav_access": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake123,
	},
	"dav_methods": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"debug_connection": []int{
		ngxEventConf | ngxConfTake1,
	},
	"debug_points": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"default_type": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"deny": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLmtConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"directio": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"directio_alignment": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"disable_symlinks": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"empty_gif": []int{
		ngxHttpLocConf | ngxConfNoArgs,
	},
	"env": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"error_log": []int{
		ngxMainConf | ngxConf1More,
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
		ngxMailMainConf | ngxMailSrvConf | ngxConf1More,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConf1More,
	},
	"error_page": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConf2More,
	},
	"etag": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"events": []int{
		ngxMainConf | ngxConfBlock | ngxConfNoArgs,
	},
	"expires": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake12,
	},
	"fastcgi_bind": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"fastcgi_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_buffering": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_buffers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"fastcgi_busy_buffers_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_cache_background_update": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_cache_bypass": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"fastcgi_cache_key": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_cache_lock": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_cache_lock_age": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_cache_lock_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_cache_max_range_offset": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_cache_methods": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"fastcgi_cache_min_uses": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_cache_path": []int{
		ngxHttpMainConf | ngxConf2More,
	},
	"fastcgi_cache_revalidate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_cache_use_stale": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"fastcgi_cache_valid": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"fastcgi_catch_stderr": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_connect_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_force_ranges": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_hide_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_ignore_client_abort": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_ignore_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"fastcgi_index": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_intercept_errors": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_keep_conn": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_limit_rate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_max_temp_file_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_next_upstream": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"fastcgi_next_upStreamtimeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_next_upStreamtries": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_no_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"fastcgi_param": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake23,
	},
	"fastcgi_pass": []int{
		ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"fastcgi_pass_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_pass_request_body": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_pass_request_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_read_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_request_buffering": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_send_lowat": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_send_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_socket_keepalive": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"fastcgi_split_path_info": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_store": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_store_access": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake123,
	},
	"fastcgi_temp_file_write_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_temp_path": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1234,
	},
	"flv": []int{
		ngxHttpLocConf | ngxConfNoArgs,
	},
	"geo": []int{
		ngxHttpMainConf | ngxConfBlock | ngxConfTake12,
		ngxStreamMainConf | ngxConfBlock | ngxConfTake12,
	},
	"geoip_city": []int{
		ngxHttpMainConf | ngxConfTake12,
		ngxStreamMainConf | ngxConfTake12,
	},
	"geoip_country": []int{
		ngxHttpMainConf | ngxConfTake12,
		ngxStreamMainConf | ngxConfTake12,
	},
	"geoip_org": []int{
		ngxHttpMainConf | ngxConfTake12,
		ngxStreamMainConf | ngxConfTake12,
	},
	"geoip_proxy": []int{
		ngxHttpMainConf | ngxConfTake1,
	},
	"geoip_proxy_recursive": []int{
		ngxHttpMainConf | ngxConfFlag,
	},
	"google_perftools_profiles": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"grpc_bind": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"grpc_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_connect_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_hide_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_ignore_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"grpc_intercept_errors": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"grpc_next_upstream": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"grpc_next_upStreamtimeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_next_upStreamtries": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_pass": []int{
		ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"grpc_pass_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_read_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_send_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_set_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"grpc_socket_keepalive": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"grpc_ssl_certificate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_ssl_certificate_key": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_ssl_ciphers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_ssl_crl": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_ssl_name": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_ssl_password_file": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_ssl_protocols": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"grpc_ssl_server_name": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"grpc_ssl_session_reuse": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"grpc_ssl_trusted_certificate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"grpc_ssl_verify": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"grpc_ssl_verify_depth": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"gunzip": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"gunzip_buffers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"gzip": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfFlag,
	},
	"gzip_buffers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"gzip_comp_level": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"gzip_disable": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"gzip_Httpversion": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"gzip_min_length": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"gzip_proxied": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"gzip_static": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"gzip_types": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"gzip_vary": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"hash": []int{
		ngxHttpUpsConf | ngxConfTake12,
		ngxStreamUpsConf | ngxConfTake12,
	},
	"http": []int{
		ngxMainConf | ngxConfBlock | ngxConfNoArgs,
	},
	"http2_body_preread_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"http2_chunk_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"http2_idle_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"http2_max_concurrent_pushes": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"http2_max_concurrent_streams": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"http2_max_field_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"http2_max_header_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"http2_max_requests": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"http2_push": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"http2_push_preload": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"http2_recv_buffer_size": []int{
		ngxHttpMainConf | ngxConfTake1,
	},
	"http2_recv_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"if": []int{
		ngxHttpSrvConf | ngxHttpLocConf | ngxConfBlock | ngxConf1More,
	},
	"if_modified_since": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"ignore_invalid_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfFlag,
	},
	"image_filter": []int{
		ngxHttpLocConf | ngxConfTake123,
	},
	"image_filter_buffer": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"image_filter_interlace": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"image_filter_jpeg_quality": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"image_filter_sharpen": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"image_filter_transparency": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"image_filter_webp_quality": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"imap_auth": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConf1More,
	},
	"imap_capabilities": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConf1More,
	},
	"imap_client_buffer": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
	},
	"include": []int{
		ngxAnyConf | ngxConfTake1,
	},
	"index": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"internal": []int{
		ngxHttpLocConf | ngxConfNoArgs,
	},
	"ip_hash": []int{
		ngxHttpUpsConf | ngxConfNoArgs,
	},
	"keepalive": []int{
		ngxHttpUpsConf | ngxConfTake1,
	},
	"keepalive_disable": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"keepalive_requests": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxHttpUpsConf | ngxConfTake1,
	},
	"keepalive_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
		ngxHttpUpsConf | ngxConfTake1,
	},
	"large_client_header_buffers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake2,
	},
	"least_conn": []int{
		ngxHttpUpsConf | ngxConfNoArgs,
		ngxStreamUpsConf | ngxConfNoArgs,
	},
	"limit_conn": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake2,
	},
	"limit_conn_dry_run": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"limit_conn_log_level": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"limit_conn_status": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"limit_conn_zone": []int{
		ngxHttpMainConf | ngxConfTake2,
		ngxStreamMainConf | ngxConfTake2,
	},
	"limit_except": []int{
		ngxHttpLocConf | ngxConfBlock | ngxConf1More,
	},
	"limit_rate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"limit_rate_after": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"limit_req": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake123,
	},
	"limit_req_dry_run": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"limit_req_log_level": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"limit_req_status": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"limit_req_zone": []int{
		ngxHttpMainConf | ngxConfTake34,
	},
	"lingering_close": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"lingering_time": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"lingering_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"listen": []int{
		ngxHttpSrvConf | ngxConf1More,
		ngxMailSrvConf | ngxConf1More,
		ngxStreamSrvConf | ngxConf1More,
	},
	"load_module": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"location": []int{
		ngxHttpSrvConf | ngxHttpLocConf | ngxConfBlock | ngxConfTake12,
	},
	"lock_file": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"log_format": []int{
		ngxHttpMainConf | ngxConf2More,
		ngxStreamMainConf | ngxConf2More,
	},
	"log_not_found": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"log_subrequest": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"mail": []int{
		ngxMainConf | ngxConfBlock | ngxConfNoArgs,
	},
	"map": []int{
		ngxHttpMainConf | ngxConfBlock | ngxConfTake2,
		ngxStreamMainConf | ngxConfBlock | ngxConfTake2,
	},
	"map_hash_bucket_size": []int{
		ngxHttpMainConf | ngxConfTake1,
		ngxStreamMainConf | ngxConfTake1,
	},
	"map_hash_max_size": []int{
		ngxHttpMainConf | ngxConfTake1,
		ngxStreamMainConf | ngxConfTake1,
	},
	"master_process": []int{
		ngxMainConf | ngxDirectConf | ngxConfFlag,
	},
	"max_ranges": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"memcached_bind": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"memcached_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"memcached_connect_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"memcached_gzip_flag": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"memcached_next_upstream": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"memcached_next_upStreamtimeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"memcached_next_upStreamtries": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"memcached_pass": []int{
		ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"memcached_read_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"memcached_send_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"memcached_socket_keepalive": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"merge_slashes": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfFlag,
	},
	"min_delete_depth": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"mirror": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"mirror_request_body": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"modern_browser": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"modern_browser_value": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"mp4": []int{
		ngxHttpLocConf | ngxConfNoArgs,
	},
	"mp4_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"mp4_max_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"msie_padding": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"msie_refresh": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"multi_accept": []int{
		ngxEventConf | ngxConfFlag,
	},
	"open_file_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"open_file_cache_errors": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"open_file_cache_min_uses": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"open_file_cache_valid": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"open_log_file_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1234,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1234,
	},
	"output_buffers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"override_charset": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfFlag,
	},
	"pcre_jit": []int{
		ngxMainConf | ngxDirectConf | ngxConfFlag,
	},
	"perl": []int{
		ngxHttpLocConf | ngxHttpLmtConf | ngxConfTake1,
	},
	"perl_modules": []int{
		ngxHttpMainConf | ngxConfTake1,
	},
	"perl_require": []int{
		ngxHttpMainConf | ngxConfTake1,
	},
	"perl_set": []int{
		ngxHttpMainConf | ngxConfTake2,
	},
	"pid": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"pop3_auth": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConf1More,
	},
	"pop3_capabilities": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConf1More,
	},
	"port_in_redirect": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"postpone_output": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"preread_buffer_size": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"preread_timeout": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"protocol": []int{
		ngxMailSrvConf | ngxConfTake1,
	},
	"proxy_bind": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake12,
	},
	"proxy_buffer": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
	},
	"proxy_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_buffering": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_buffers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"proxy_busy_buffers_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_cache_background_update": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_cache_bypass": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"proxy_cache_convert_head": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_cache_key": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_cache_lock": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_cache_lock_age": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_cache_lock_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_cache_max_range_offset": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_cache_methods": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"proxy_cache_min_uses": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_cache_path": []int{
		ngxHttpMainConf | ngxConf2More,
	},
	"proxy_cache_revalidate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_cache_use_stale": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"proxy_cache_valid": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"proxy_connect_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_cookie_domain": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"proxy_cookie_path": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"proxy_download_rate": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_force_ranges": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_headers_hash_bucket_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_headers_hash_max_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_hide_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_Httpversion": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_ignore_client_abort": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_ignore_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"proxy_intercept_errors": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_limit_rate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_max_temp_file_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_method": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_next_upstream": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"proxy_next_upStreamtimeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_next_upStreamtries": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_no_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"proxy_pass": []int{
		ngxHttpLocConf | ngxHttpLifConf | ngxHttpLmtConf | ngxConfTake1,
		ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_pass_error_message": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfFlag,
	},
	"proxy_pass_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_pass_request_body": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_pass_request_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_protocol": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"proxy_protocol_timeout": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_read_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_redirect": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"proxy_request_buffering": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"proxy_requests": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_responses": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_send_lowat": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_send_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_set_body": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_set_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"proxy_socket_keepalive": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"proxy_ssl": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"proxy_ssl_certificate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_ssl_certificate_key": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_ssl_ciphers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_ssl_crl": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_ssl_name": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_ssl_password_file": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_ssl_protocols": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConf1More,
	},
	"proxy_ssl_server_name": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"proxy_ssl_session_reuse": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"proxy_ssl_trusted_certificate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_ssl_verify": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"proxy_ssl_verify_depth": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_store": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_store_access": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake123,
	},
	"proxy_temp_file_write_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"proxy_temp_path": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1234,
	},
	"proxy_timeout": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"proxy_upload_rate": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"random": []int{
		ngxHttpUpsConf | ngxConfNoArgs | ngxConfTake12,
		ngxStreamUpsConf | ngxConfNoArgs | ngxConfTake12,
	},
	"random_index": []int{
		ngxHttpLocConf | ngxConfFlag,
	},
	"read_ahead": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"real_ip_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"real_ip_recursive": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"recursive_error_pages": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"referer_hash_bucket_size": []int{
		ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"referer_hash_max_size": []int{
		ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"request_pool_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"reset_timedout_connection": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"resolver": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
		ngxMailMainConf | ngxMailSrvConf | ngxConf1More,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConf1More,
	},
	"resolver_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"return": []int{
		ngxHttpSrvConf | ngxHttpSifConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake12,
		ngxStreamSrvConf | ngxConfTake1,
	},
	"rewrite": []int{
		ngxHttpSrvConf | ngxHttpSifConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake23,
	},
	"rewrite_log": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpSifConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfFlag,
	},
	"root": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"satisfy": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_bind": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"scgi_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_buffering": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_buffers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"scgi_busy_buffers_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_cache_background_update": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_cache_bypass": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"scgi_cache_key": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_cache_lock": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_cache_lock_age": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_cache_lock_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_cache_max_range_offset": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_cache_methods": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"scgi_cache_min_uses": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_cache_path": []int{
		ngxHttpMainConf | ngxConf2More,
	},
	"scgi_cache_revalidate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_cache_use_stale": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"scgi_cache_valid": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"scgi_connect_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_force_ranges": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_hide_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_ignore_client_abort": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_ignore_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"scgi_intercept_errors": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_limit_rate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_max_temp_file_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_next_upstream": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"scgi_next_upStreamtimeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_next_upStreamtries": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_no_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"scgi_param": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake23,
	},
	"scgi_pass": []int{
		ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"scgi_pass_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_pass_request_body": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_pass_request_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_read_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_request_buffering": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_send_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_socket_keepalive": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"scgi_store": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_store_access": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake123,
	},
	"scgi_temp_file_write_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"scgi_temp_path": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1234,
	},
	"secure_link": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"secure_link_md5": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"secure_link_secret": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"send_lowat": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"send_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"sendfile": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfFlag,
	},
	"sendfile_max_chunk": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"server": []int{
		ngxHttpMainConf | ngxConfBlock | ngxConfNoArgs,
		ngxHttpUpsConf | ngxConf1More,
		ngxMailMainConf | ngxConfBlock | ngxConfNoArgs,
		ngxStreamMainConf | ngxConfBlock | ngxConfNoArgs,
		ngxStreamUpsConf | ngxConf1More,
	},
	"server_name": []int{
		ngxHttpSrvConf | ngxConf1More,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
	},
	"server_name_in_redirect": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"server_names_hash_bucket_size": []int{
		ngxHttpMainConf | ngxConfTake1,
	},
	"server_names_hash_max_size": []int{
		ngxHttpMainConf | ngxConfTake1,
	},
	"server_tokens": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"set": []int{
		ngxHttpSrvConf | ngxHttpSifConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake2,
	},
	"set_real_ip_from": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"slice": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"smtp_auth": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConf1More,
	},
	"smtp_capabilities": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConf1More,
	},
	"smtp_client_buffer": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
	},
	"smtp_greeting_delay": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
	},
	"source_charset": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"spdy_chunk_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"spdy_headers_comp": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"split_clients": []int{
		ngxHttpMainConf | ngxConfBlock | ngxConfTake2,
		ngxStreamMainConf | ngxConfBlock | ngxConfTake2,
	},
	"ssi": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfFlag,
	},
	"ssi_last_modified": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"ssi_min_file_chunk": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"ssi_silent_errors": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"ssi_types": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"ssi_value_length": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"ssl": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfFlag,
		ngxMailMainConf | ngxMailSrvConf | ngxConfFlag,
	},
	"ssl_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"ssl_certificate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_certificate_key": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_ciphers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_client_certificate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_crl": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_dhparam": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_early_data": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfFlag,
	},
	"ssl_ecdh_curve": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_engine": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"ssl_handshake_timeout": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_password_file": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_prefer_server_ciphers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfFlag,
		ngxMailMainConf | ngxMailSrvConf | ngxConfFlag,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"ssl_preread": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"ssl_protocols": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConf1More,
		ngxMailMainConf | ngxMailSrvConf | ngxConf1More,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConf1More,
	},
	"ssl_session_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake12,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake12,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake12,
	},
	"ssl_session_ticket_key": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_session_tickets": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfFlag,
		ngxMailMainConf | ngxMailSrvConf | ngxConfFlag,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"ssl_session_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_stapling": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfFlag,
	},
	"ssl_stapling_file": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"ssl_stapling_responder": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
	},
	"ssl_stapling_verify": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfFlag,
	},
	"ssl_trusted_certificate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_verify_client": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"ssl_verify_depth": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfTake1,
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"starttls": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
	},
	"stream": []int{
		ngxMainConf | ngxConfBlock | ngxConfNoArgs,
	},
	"stub_status": []int{
		ngxHttpSrvConf | ngxHttpLocConf | ngxConfNoArgs | ngxConfTake1,
	},
	"sub_filter": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"sub_filter_last_modified": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"sub_filter_once": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"sub_filter_types": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"subrequest_output_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"tcp_nodelay": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"tcp_nopush": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"thread_pool": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake23,
	},
	"timeout": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfTake1,
	},
	"timer_resolution": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"try_files": []int{
		ngxHttpSrvConf | ngxHttpLocConf | ngxConf2More,
	},
	"types": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfBlock | ngxConfNoArgs,
	},
	"types_hash_bucket_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"types_hash_max_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"underscores_in_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxConfFlag,
	},
	"uninitialized_variable_warn": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpSifConf | ngxHttpLocConf | ngxHttpLifConf | ngxConfFlag,
	},
	"upstream": []int{
		ngxHttpMainConf | ngxConfBlock | ngxConfTake1,
		ngxStreamMainConf | ngxConfBlock | ngxConfTake1,
	},
	"use": []int{
		ngxEventConf | ngxConfTake1,
	},
	"user": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake12,
	},
	"userid": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"userid_domain": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"userid_expires": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"userid_mark": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"userid_name": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"userid_p3p": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"userid_path": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"userid_service": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_bind": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"uwsgi_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_buffering": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_buffers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"uwsgi_busy_buffers_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_cache_background_update": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_cache_bypass": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"uwsgi_cache_key": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_cache_lock": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_cache_lock_age": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_cache_lock_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_cache_max_range_offset": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_cache_methods": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"uwsgi_cache_min_uses": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_cache_path": []int{
		ngxHttpMainConf | ngxConf2More,
	},
	"uwsgi_cache_revalidate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_cache_use_stale": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"uwsgi_cache_valid": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"uwsgi_connect_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_force_ranges": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_hide_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_ignore_client_abort": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_ignore_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"uwsgi_intercept_errors": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_limit_rate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_max_temp_file_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_modifier1": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_modifier2": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_next_upstream": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"uwsgi_next_upStreamtimeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_next_upStreamtries": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_no_cache": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"uwsgi_param": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake23,
	},
	"uwsgi_pass": []int{
		ngxHttpLocConf | ngxHttpLifConf | ngxConfTake1,
	},
	"uwsgi_pass_header": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_pass_request_body": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_pass_request_headers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_read_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_request_buffering": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_send_timeout": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_socket_keepalive": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_ssl_certificate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_ssl_certificate_key": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_ssl_ciphers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_ssl_crl": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_ssl_name": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_ssl_password_file": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_ssl_protocols": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"uwsgi_ssl_server_name": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_ssl_session_reuse": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_ssl_trusted_certificate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_ssl_verify": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"uwsgi_ssl_verify_depth": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_store": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_store_access": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake123,
	},
	"uwsgi_temp_file_write_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"uwsgi_temp_path": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1234,
	},
	"valid_referers": []int{
		ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"variables_hash_bucket_size": []int{
		ngxHttpMainConf | ngxConfTake1,
		ngxStreamMainConf | ngxConfTake1,
	},
	"variables_hash_max_size": []int{
		ngxHttpMainConf | ngxConfTake1,
		ngxStreamMainConf | ngxConfTake1,
	},
	"worker_aio_requests": []int{
		ngxEventConf | ngxConfTake1,
	},
	"worker_connections": []int{
		ngxEventConf | ngxConfTake1,
	},
	"worker_cpu_affinity": []int{
		ngxMainConf | ngxDirectConf | ngxConf1More,
	},
	"worker_priority": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"worker_processes": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"worker_rlimit_core": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"worker_rlimit_nofile": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"worker_shutdown_timeout": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"working_directory": []int{
		ngxMainConf | ngxDirectConf | ngxConfTake1,
	},
	"xclient": []int{
		ngxMailMainConf | ngxMailSrvConf | ngxConfFlag,
	},
	"xml_entities": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"xslt_last_modified": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"xslt_param": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"xslt_string_param": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"xslt_stylesheet": []int{
		ngxHttpLocConf | ngxConf1More,
	},
	"xslt_types": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"zone": []int{
		ngxHttpUpsConf | ngxConfTake12,
		ngxStreamUpsConf | ngxConfTake12,
	},

	// nginx+ directives [definitions inferred from docs]
	"api": []int{
		ngxHttpLocConf | ngxConfNoArgs | ngxConfTake1,
	},
	"auth_jwt": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"auth_jwt_claim_set": []int{
		ngxHttpMainConf | ngxConf2More,
	},
	"auth_jwt_header_set": []int{
		ngxHttpMainConf | ngxConf2More,
	},
	"auth_jwt_key_file": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"auth_jwt_key_request": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"auth_jwt_leeway": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"f4f": []int{
		ngxHttpLocConf | ngxConfNoArgs,
	},
	"f4f_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"fastcgi_cache_purge": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"health_check": []int{
		ngxHttpLocConf | ngxConfAny,
		ngxStreamSrvConf | ngxConfAny,
	},
	"health_check_timeout": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"hls": []int{
		ngxHttpLocConf | ngxConfNoArgs,
	},
	"hls_buffers": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake2,
	},
	"hls_forward_args": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"hls_fragment": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"hls_mp4_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"hls_mp4_max_buffer_size": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"js_access": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"js_content": []int{
		ngxHttpLocConf | ngxHttpLmtConf | ngxConfTake1,
	},
	"js_filter": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"js_include": []int{
		ngxHttpMainConf | ngxConfTake1,
		ngxStreamMainConf | ngxConfTake1,
	},
	"js_path": []int{
		ngxHttpMainConf | ngxConfTake1,
	},
	"js_preread": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"js_set": []int{
		ngxHttpMainConf | ngxConfTake2,
		ngxStreamMainConf | ngxConfTake2,
	},
	"keyval": []int{
		ngxHttpMainConf | ngxConfTake3,
		ngxStreamMainConf | ngxConfTake3,
	},
	"keyval_zone": []int{
		ngxHttpMainConf | ngxConf1More,
		ngxStreamMainConf | ngxConf1More,
	},
	"least_time": []int{
		ngxHttpUpsConf | ngxConfTake12,
		ngxStreamUpsConf | ngxConfTake12,
	},
	"limit_zone": []int{
		ngxHttpMainConf | ngxConfTake3,
	},
	"match": []int{
		ngxHttpMainConf | ngxConfBlock | ngxConfTake1,
		ngxStreamMainConf | ngxConfBlock | ngxConfTake1,
	},
	"memcached_force_ranges": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfFlag,
	},
	"mp4_limit_rate": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"mp4_limit_rate_after": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"ntlm": []int{
		ngxHttpUpsConf | ngxConfNoArgs,
	},
	"proxy_cache_purge": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"queue": []int{
		ngxHttpUpsConf | ngxConfTake12,
	},
	"scgi_cache_purge": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"session_log": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake1,
	},
	"session_log_format": []int{
		ngxHttpMainConf | ngxConf2More,
	},
	"session_log_zone": []int{
		ngxHttpMainConf | ngxConfTake23 | ngxConfTake4 | ngxConfTake5 | ngxConfTake6,
	},
	"state": []int{
		ngxHttpUpsConf | ngxConfTake1,
		ngxStreamUpsConf | ngxConfTake1,
	},
	"status": []int{
		ngxHttpLocConf | ngxConfNoArgs,
	},
	"status_format": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConfTake12,
	},
	"status_zone": []int{
		ngxHttpSrvConf | ngxConfTake1,
		ngxStreamSrvConf | ngxConfTake1,
		ngxHttpLocConf | ngxConfTake1,
		ngxHttpLifConf | ngxConfTake1,
	},
	"sticky": []int{
		ngxHttpUpsConf | ngxConf1More,
	},
	"sticky_cookie_insert": []int{
		ngxHttpUpsConf | ngxConfTake1234,
	},
	"upStreamconf": []int{
		ngxHttpLocConf | ngxConfNoArgs,
	},
	"uwsgi_cache_purge": []int{
		ngxHttpMainConf | ngxHttpSrvConf | ngxHttpLocConf | ngxConf1More,
	},
	"zone_sync": []int{
		ngxStreamSrvConf | ngxConfNoArgs,
	},
	"zone_sync_buffers": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake2,
	},
	"zone_sync_connect_retry_interval": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_connect_timeout": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_interval": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_recv_buffer_size": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_server": []int{
		ngxStreamSrvConf | ngxConfTake12,
	},
	"zone_sync_ssl": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"zone_sync_ssl_certificate": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_ssl_certificate_key": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_ssl_ciphers": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_ssl_crl": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_ssl_name": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_ssl_password_file": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_ssl_protocols": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConf1More,
	},
	"zone_sync_ssl_server_name": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"zone_sync_ssl_trusted_certificate": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_ssl_verify": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfFlag,
	},
	"zone_sync_ssl_verify_depth": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
	"zone_sync_timeout": []int{
		ngxStreamMainConf | ngxStreamSrvConf | ngxConfTake1,
	},
}
