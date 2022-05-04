/*
Package tlsLayer provides facilities for tls, including sniffing.
*/
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
	etStrMap = make(map[int]string)
	etStrMap[et_server_name] = etstr_server_name
	etStrMap[et_max_fragment_length] = etstr_max_fragment_length
	etStrMap[et_status_request] = etstr_status_request
	etStrMap[et_supported_groups] = etstr_supported_groups
	etStrMap[et_signature_algorithms] = etstr_signature_algorithms
	etStrMap[et_use_srtp] = etstr_use_srtp
	etStrMap[et_heartbeat] = etstr_heartbeat
	etStrMap[et_application_layer_protocol_negotiation] = etstr_application_layer_protocol_negotiation
	etStrMap[et_signed_certificate_timestamp] = etstr_signed_certificate_timestamp
	etStrMap[et_client_certificate_type] = etstr_client_certificate_type
	etStrMap[et_server_certificate_type] = etstr_server_certificate_type
	etStrMap[et_padding] = etstr_padding
	etStrMap[et_pre_shared_key] = etstr_pre_shared_key
	etStrMap[et_early_data] = etstr_early_data
	etStrMap[et_supported_versions] = etstr_supported_versions
	etStrMap[et_cookie] = etstr_cookie
	etStrMap[et_psk_key_exchange_modes] = etstr_psk_key_exchange_modes
	etStrMap[et_certificate_authorities] = etstr_certificate_authorities
	etStrMap[et_oid_filters] = etstr_oid_filters
	etStrMap[et_post_handshake_auth] = etstr_post_handshake_auth
	etStrMap[et_signature_algorithms_cert] = etstr_signature_algorithms_cert
	etStrMap[et_key_share] = etstr_key_share
}
