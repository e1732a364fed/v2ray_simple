package httpLayer

import "testing"

func TestGetPath(t *testing.T) {
	str1 := "GET /sdfdsffs.html HTTP/0.9\r\n"
	p1 := GetRequestPATH_from_Bytes([]byte(str1))
	if p1 != "/sdfdsffs.html" {
		t.Log("get path failed", p1, len(str1), len("/sdfdsffs.html"))
		t.FailNow()
	}

	str2 := "CONNECT /sdfdsffs.html HTTP/0.9\r\n"
	p2 := GetRequestPATH_from_Bytes([]byte(str2))
	if p2 != "/sdfdsffs.html" {
		t.Log("get path failed", p2)
		t.FailNow()
	}

	str3 := "GET /x HTTP/0.9\r"
	p3 := GetRequestPATH_from_Bytes([]byte(str3))
	if p3 == "/x" { //尾缀长度不够
		t.Log("get path failed", len(str3), p3)
		t.FailNow()
	}

	str3 = "GET /x HTTP/0.9\r\n"
	p3 = GetRequestPATH_from_Bytes([]byte(str3))
	if p3 != "/x" {
		t.Log("get path failed", len(str3), p3)
		t.FailNow()
	}
}
