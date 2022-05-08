package tlsLayer

var etStrMap map[int]string

const (
	et_server_name                            = 0
	et_max_fragment_length                    = 1
	et_status_request                         = 5
	et_supported_groups                       = 10
	et_signature_algorithms                   = 13
	et_use_srtp                               = 14
	et_heartbeat                              = 15
	et_application_layer_protocol_negotiation = 16
	et_signed_certificate_timestamp           = 18
	et_client_certificate_type                = 19
	et_server_certificate_type                = 20
	et_padding                                = 21
	et_pre_shared_key                         = 41
	et_early_data                             = 42
	et_supported_versions                     = 43
	et_cookie                                 = 44
	et_psk_key_exchange_modes                 = 45
	et_certificate_authorities                = 47
	et_oid_filters                            = 48
	et_post_handshake_auth                    = 49
	et_signature_algorithms_cert              = 50
	et_key_share                              = 51
)

const (
	etstr_server_name                            = "server_name "
	etstr_max_fragment_length                    = "max_fragment_length"
	etstr_status_request                         = "status_request"
	etstr_supported_groups                       = "supported_groups"
	etstr_signature_algorithms                   = "signature_algorithms"
	etstr_use_srtp                               = "use_srtp"
	etstr_heartbeat                              = "heartbeat"
	etstr_application_layer_protocol_negotiation = "application_layer_protocol_negotiation"
	etstr_signed_certificate_timestamp           = "signed_certificate_timestamp"
	etstr_client_certificate_type                = "client_certificate_type"
	etstr_server_certificate_type                = "server_certificate_type"
	etstr_padding                                = "padding"
	etstr_pre_shared_key                         = "pre_shared_key"
	etstr_early_data                             = "early_data"
	etstr_supported_versions                     = "supported_versions"
	etstr_cookie                                 = "cookie"
	etstr_psk_key_exchange_modes                 = "psk_key_exchange_modes"
	etstr_certificate_authorities                = "certificate_authorities"
	etstr_oid_filters                            = "oid_filters"
	etstr_post_handshake_auth                    = "post_handshake_auth"
	etstr_signature_algorithms_cert              = "signature_algorithms_cert"
	etstr_key_share                              = "key_share"
)

func init() {
	etStrMap = map[int]string{
		et_server_name:          etstr_server_name,
		et_max_fragment_length:  etstr_max_fragment_length,
		et_status_request:       etstr_status_request,
		et_supported_groups:     etstr_supported_groups,
		et_signature_algorithms: etstr_signature_algorithms,
		et_use_srtp:             etstr_use_srtp,
		et_heartbeat:            etstr_heartbeat,
		et_application_layer_protocol_negotiation: etstr_application_layer_protocol_negotiation,
		et_signed_certificate_timestamp:           etstr_signed_certificate_timestamp,
		et_client_certificate_type:                etstr_client_certificate_type,
		et_server_certificate_type:                etstr_server_certificate_type,
		et_padding:                                etstr_padding,
		et_pre_shared_key:                         etstr_pre_shared_key,
		et_early_data:                             etstr_early_data,
		et_supported_versions:                     etstr_supported_versions,
		et_cookie:                                 etstr_cookie,
		et_psk_key_exchange_modes:                 etstr_psk_key_exchange_modes,
		et_certificate_authorities:                etstr_certificate_authorities,
		et_oid_filters:                            etstr_oid_filters,
		et_post_handshake_auth:                    etstr_post_handshake_auth,
		et_signature_algorithms_cert:              etstr_signature_algorithms_cert,
		et_key_share:                              etstr_key_share,
	}

}
