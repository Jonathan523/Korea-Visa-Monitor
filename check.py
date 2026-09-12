import os

import requests
from bs4 import BeautifulSoup


URL = "https://www.visa.go.kr/openPage.do?MENU_ID=10301"

# ── 配置（全部来自环境变量）──────────────────────────────
# 证件信息：护照号码 / 英文姓名（大写）/ 出生日期（YYYY-MM-DD）
PASSPORT_NUMBER = os.environ.get("VISA_PASSPORT_NUMBER", "")
ENGLISH_NAME = os.environ.get("VISA_ENGLISH_NAME", "")
BIRTHDAY = os.environ.get("VISA_BIRTHDAY", "")
# ────────────────────────────────────────────────────────


def query_visa_status(passport_number, english_name, birthday):
    session = requests.Session()

    headers = {
        "User-Agent": (
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
            "AppleWebKit/537.36 (KHTML, like Gecko) "
            "Chrome/151.0.0.0 Safari/537.36"
        ),
        "Accept": (
            "text/html,application/xhtml+xml,application/xml;q=0.9,"
            "image/avif,image/webp,image/apng,*/*;q=0.8"
        ),
        "Accept-Language": "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
        "DNT": "1",
    }

    # 先访问页面，自动获得 JSESSIONID / WMONID
    response = session.get(
        URL,
        headers=headers,
        timeout=20
    )
    response.raise_for_status()

    data = {
        "CMM_TEST_VAL": "test",

        # 护照号码
        "sBUSI_GB": "PASS_NO",
        "sBUSI_GBNO": passport_number,
        "ssBUSI_GBNO": passport_number,

        # 驻外使领馆签证申请
        "pRADIOSEARCH": "gb03",

        # 英文姓名
        "sEK_NM": english_name.upper(),

        # YYYY-MM-DD
        "sFROMDATE": birthday,

        "sMainPopUpGB": "main",
        "TRAN_TYPE": "ComSubmit",
        "SE_FLAG_YN": "",
        "LANG_TYPE": "CH",
    }

    post_headers = {
        **headers,
        "Content-Type": "application/x-www-form-urlencoded",
        "Origin": "https://www.visa.go.kr",
        "Referer": URL,
    }

    response = session.post(
        URL,
        headers=post_headers,
        data=data,
        timeout=20
    )
    response.raise_for_status()

    response.encoding = "utf-8"

    return parse_visa_result(response.text)


def get_element_text(parent, element_id):
    element = parent.find(id=element_id)

    if element is None:
        return None

    text = element.get_text(
        " ",
        strip=True
    )

    if not text:
        return None

    return text


def parse_visa_result(html):
    soup = BeautifulSoup(
        html,
        "html.parser"
    )

    # gb03 的查询结果区域
    result_area = soup.find(
        "div",
        id="result3_2"
    )

    if result_area is None:
        return {
            "found": False,
            "message": "未找到 result3_2 查询结果区域"
        }

    application_number = get_element_text(
        result_area,
        "ONLINE_APPL_NO"
    )

    entry_purpose = get_element_text(
        result_area,
        "ENTRY_PURPOSE"
    )

    status = get_element_text(
        result_area,
        "PROC_STS_CDNM_1"
    )

    if application_number is None:
        return {
            "found": False,
            "message": "未查询到签证申请记录"
        }

    return {
        "found": True,
        "application_number": application_number,
        "entry_purpose": entry_purpose,
        "status": status,
    }


def print_result(result):
    if not result["found"]:
        print("查询失败：")
        print(result["message"])
        return

    print("签证查询成功")
    print("=" * 40)

    print(
        "申请编号：",
        result["application_number"]
    )

    print(
        "入境目的：",
        result["entry_purpose"]
    )

    print(
        "当前状态：",
        result["status"]
    )


def main():
    result = query_visa_status(
        PASSPORT_NUMBER,
        ENGLISH_NAME,
        BIRTHDAY
    )

    print_result(result)


if __name__ == "__main__":
    main()
