package httpLayer

import "testing"

func TestGetPath(t *testing.T) {

	str1 := "GET /sdfdsffs.html HTTP/0.9\r\n"
	_, method, p1, _, falreason := ParseH1Request([]byte(str1), false)
	if p1 != "/sdfdsffs.html" || method != "GET" {
		t.Log("get path failed", p1, len(str1), falreason, len("/sdfdsffs.html"))
		t.FailNow()
	}

	str2 := "CONNECT /sdfdsffs.html HTTP/0.9\r\n"
	_, _, p2, _, falreason := ParseH1Request([]byte(str2), false)
	if p2 != "/sdfdsffs.html" {
		t.Log("get path failed", falreason, p2)
		t.FailNow()
	}

	str3 := "GET /x HTTP/0.9\r"
	_, _, p3, _, falreason := ParseH1Request([]byte(str3), false)
	if p3 == "/x" { //尾缀长度不够
		t.Log("get path failed", len(str3), falreason, p3)
		t.FailNow()
	}

	str3 = "GET /x HTTP/0.9\r\n"
	_, _, p3, _, falreason = ParseH1Request([]byte(str3), false)
	if p3 != "/x" {
		t.Log("get path failed", len(str3), falreason, p3)
		t.FailNow()
	}

	requestStr := "http://image.baidu.com/search/index?tn=baiduimage&ps=1&ct=111111111&lm=-1&cl=2&nc=1&ie=utf-8&dyTabStr=adfdfadfdafsdfadfafdafadfa&word=sdf"

	str4 := "GET " + requestStr + " HTTP/1.1\r\n"

	_, _, p4, _, failreason := ParseH1Request([]byte(str4), true)
	if p4 != requestStr {
		t.Log("get path failed", len(str4), failreason, p4)
		t.FailNow()
	}

}
