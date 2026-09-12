import json
import os
import sys
from datetime import datetime, time as dtime, timedelta, timezone

import requests

# 保证无论从哪个目录被调用，都能 import 同目录下的 check.py
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from check import (
    PASSPORT_NUMBER,
    ENGLISH_NAME,
    BIRTHDAY,
    query_visa_status,
)

# ── 配置（全部来自环境变量）─────────────────────────────
BASE_DIR = os.path.dirname(os.path.abspath(__file__))

TZ = timezone(timedelta(hours=8))  # UTC+8


def _parse_window(name, default_str):
    """从环境变量读取 HH:MM 格式的时间；无效时回退到默认值。"""
    raw = os.environ.get(name, default_str)

    try:
        hour, minute = raw.split(":")
        return dtime(int(hour), int(minute))
    except (TypeError, ValueError):
        print(
            f"警告：环境变量 {name} 无效（应为 HH:MM），使用默认值 {default_str}",
            file=sys.stderr,
        )
        hour, minute = default_str.split(":")
        return dtime(int(hour), int(minute))


# 每天查询窗口（UTC+8），不含结束时间
WINDOW_START = _parse_window("VISA_WINDOW_START", "08:00")
WINDOW_END = _parse_window("VISA_WINDOW_END", "20:00")

# 推送渠道：pushdeer 或 serverchan（默认 pushdeer）
PUSH_CHANNEL = os.environ.get("VISA_PUSH_CHANNEL", "pushdeer").strip().lower()

# Server酱 SendKey，在 https://sct.ftqq.com 获取
SERVERCHAN_KEY = os.environ.get("VISA_SERVERCHAN_KEY", "")

# PushDeer PushKey，在 https://www.pushdeer.com 获取
PUSHDEER_KEY = os.environ.get("VISA_PUSHDEER_KEY", "")
PUSHDEER_ENDPOINT = os.environ.get(
    "VISA_PUSHDEER_ENDPOINT",
    "https://api2.pushdeer.com/message/push",
)

# 状态存储方式：local、s3 或 upstash（默认 local，保持原有行为）
STATE_STORAGE = os.environ.get("VISA_STATE_STORAGE", "local").strip().lower()

# 本地状态文件路径
STATE_FILE = os.environ.get(
    "VISA_STATE_FILE",
    os.path.join(BASE_DIR, "visa_state.json"),
)

# S3 状态对象配置。凭据使用 boto3 支持的标准 AWS 环境变量：
# AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN（可选）。
S3_BUCKET = os.environ.get("VISA_S3_BUCKET", "").strip()
S3_KEY = os.environ.get("VISA_S3_KEY", "visa_state.json").strip()
S3_REGION = os.environ.get("VISA_S3_REGION", "").strip()
S3_ENDPOINT_URL = os.environ.get("VISA_S3_ENDPOINT_URL", "").strip()

# Upstash Redis REST 配置。使用 HTTP 接口可避免额外安装 Redis 客户端。
UPSTASH_REDIS_REST_URL = os.environ.get(
    "UPSTASH_REDIS_REST_URL", ""
).strip().rstrip("/")
UPSTASH_REDIS_REST_TOKEN = os.environ.get(
    "UPSTASH_REDIS_REST_TOKEN", ""
).strip()
UPSTASH_KEY = os.environ.get(
    "VISA_UPSTASH_KEY", "krvisa:visa_state"
).strip()
# ────────────────────────────────────────────────────────


def send_push(text, desp):
    """按 VISA_PUSH_CHANNEL 指定的渠道发送一条通知。"""
    if PUSH_CHANNEL == "serverchan":
        send_serverchan(text, desp)
    elif PUSH_CHANNEL == "pushdeer":
        send_pushdeer(text, desp)
    else:
        raise RuntimeError(
            f"未知推送渠道：{PUSH_CHANNEL}（可选：pushdeer / serverchan）"
        )


def send_serverchan(text, desp):
    """通过 Server酱（方糖）发送一条通知。"""
    if not SERVERCHAN_KEY:
        raise RuntimeError("未配置 VISA_SERVERCHAN_KEY")

    from serverchan_sdk import sc_send

    response = sc_send(
        SERVERCHAN_KEY,
        text,
        desp,
        {"tags": "签证监控|图片"},
    )

    if response.get("code") != 0:
        raise RuntimeError(f"Server酱返回错误：{response}")


def send_pushdeer(text, desp):
    """通过 PushDeer 发送一条通知。"""
    if not PUSHDEER_KEY:
        raise RuntimeError("未配置 VISA_PUSHDEER_KEY")

    response = requests.post(
        PUSHDEER_ENDPOINT,
        data={
            "pushkey": PUSHDEER_KEY,
            "text": text,
            "desp": desp,
            "type": "markdown",
        },
        timeout=20,
    )
    response.raise_for_status()

    payload = response.json()

    if payload.get("code") != 0:
        raise RuntimeError(f"PushDeer返回错误：{payload}")


def format_result(result):
    """把查询结果格式化成便于阅读的文本。"""
    if not result["found"]:
        return f"未查询到记录：{result['message']}"

    return "\n".join(
        [
            f"申请编号：{result['application_number']}",
            f"入境目的：{result['entry_purpose']}",
            f"当前状态：{result['status']}",
        ]
    )


def state_string(result):
    """把查询结果压缩成一条规范字符串，用于前后对比。

    只要任何字段字符串不一致，就视为状态有变化。
    """
    if not result["found"]:
        return f"not_found|{result.get('message', '')}"

    return "|".join(
        [
            "found",
            str(result["application_number"]),
            str(result["entry_purpose"]),
            str(result["status"]),
        ]
    )


def _s3_client():
    """按环境变量创建 S3 客户端；仅在选择 S3 存储时加载 boto3。"""
    try:
        import boto3
    except ImportError as exc:
        raise RuntimeError("使用 S3 状态存储需要安装 boto3") from exc

    options = {}

    if S3_REGION:
        options["region_name"] = S3_REGION

    if S3_ENDPOINT_URL:
        options["endpoint_url"] = S3_ENDPOINT_URL

    return boto3.client("s3", **options)


def validate_state_storage_config():
    """尽早检查状态存储配置，避免查询完成后才发现配置错误。"""
    if STATE_STORAGE not in {"local", "s3", "upstash"}:
        raise RuntimeError(
            f"未知状态存储方式：{STATE_STORAGE}（可选：local / s3 / upstash）"
        )

    if STATE_STORAGE == "s3":
        if not S3_BUCKET:
            raise RuntimeError("使用 S3 状态存储时必须配置 VISA_S3_BUCKET")
        if not S3_KEY:
            raise RuntimeError("VISA_S3_KEY 不能为空")

    if STATE_STORAGE == "upstash":
        if not UPSTASH_REDIS_REST_URL:
            raise RuntimeError(
                "使用 Upstash 状态存储时必须配置 UPSTASH_REDIS_REST_URL"
            )
        if not UPSTASH_REDIS_REST_TOKEN:
            raise RuntimeError(
                "使用 Upstash 状态存储时必须配置 UPSTASH_REDIS_REST_TOKEN"
            )
        if not UPSTASH_KEY:
            raise RuntimeError("VISA_UPSTASH_KEY 不能为空")


def load_local_state():
    """读取本地状态；文件不存在或损坏时返回 None。"""
    try:
        with open(STATE_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return None


def save_local_state(state):
    """原子写入本地状态，避免 cron 并发运行时读到半个文件。"""
    tmp = STATE_FILE + ".tmp"

    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(state, f, ensure_ascii=False, indent=2)

    os.replace(tmp, STATE_FILE)


def load_s3_state():
    """读取 S3 状态；对象不存在或内容损坏时返回 None。"""
    try:
        response = _s3_client().get_object(Bucket=S3_BUCKET, Key=S3_KEY)
    except Exception as exc:
        error = getattr(exc, "response", {}).get("Error", {})
        if str(error.get("Code")) in {"NoSuchKey", "404", "NotFound"}:
            return None

        raise RuntimeError(f"从 S3 读取状态失败：{exc}") from exc

    try:
        return json.loads(response["Body"].read().decode("utf-8"))
    except (KeyError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        print(f"警告：S3 状态对象内容无效，将按无状态处理：{exc}", file=sys.stderr)
        return None


def save_s3_state(state):
    """将当前状态写入 S3 对象。"""
    body = json.dumps(state, ensure_ascii=False, indent=2).encode("utf-8")

    try:
        _s3_client().put_object(
            Bucket=S3_BUCKET,
            Key=S3_KEY,
            Body=body,
            ContentType="application/json; charset=utf-8",
        )
    except Exception as exc:
        raise RuntimeError(f"向 S3 保存状态失败：{exc}") from exc


def _upstash_command(command):
    """通过 Upstash REST API 执行一条 Redis 命令。"""
    try:
        response = requests.post(
            UPSTASH_REDIS_REST_URL,
            headers={
                "Authorization": f"Bearer {UPSTASH_REDIS_REST_TOKEN}",
                "Content-Type": "application/json",
            },
            json=command,
            timeout=20,
        )
        response.raise_for_status()
        payload = response.json()
    except (requests.RequestException, ValueError) as exc:
        raise RuntimeError(f"访问 Upstash 状态存储失败：{exc}") from exc

    if "error" in payload:
        raise RuntimeError(f"Upstash 返回错误：{payload['error']}")
    if "result" not in payload:
        raise RuntimeError("Upstash 返回了无法识别的响应")

    return payload["result"]


def load_upstash_state():
    """从 Upstash 读取状态；键不存在或内容损坏时返回 None。"""
    value = _upstash_command(["GET", UPSTASH_KEY])

    if value is None:
        return None

    try:
        return json.loads(value)
    except (TypeError, json.JSONDecodeError) as exc:
        print(
            f"警告：Upstash 状态内容无效，将按无状态处理：{exc}",
            file=sys.stderr,
        )
        return None


def save_upstash_state(state):
    """将当前状态写入 Upstash。"""
    value = json.dumps(state, ensure_ascii=False)
    result = _upstash_command(["SET", UPSTASH_KEY, value])

    if result != "OK":
        raise RuntimeError(f"Upstash 写入状态失败：返回值为 {result!r}")


def load_state():
    """从所选存储后端读取上次保存的状态。"""
    if STATE_STORAGE == "s3":
        return load_s3_state()
    if STATE_STORAGE == "upstash":
        return load_upstash_state()
    return load_local_state()


def save_state(status_text, summary, now):
    """将当前状态写入所选存储后端。"""
    state = {
        "status_text": status_text,
        "summary": summary,
        "updated_at": now.isoformat(),
    }

    if STATE_STORAGE == "s3":
        save_s3_state(state)
    elif STATE_STORAGE == "upstash":
        save_upstash_state(state)
    else:
        save_local_state(state)


def main():
    if not all([PASSPORT_NUMBER, ENGLISH_NAME, BIRTHDAY]):
        print(
            "错误：请通过环境变量设置 "
            "VISA_PASSPORT_NUMBER / VISA_ENGLISH_NAME / VISA_BIRTHDAY",
            file=sys.stderr,
        )
        sys.exit(2)

    try:
        validate_state_storage_config()
    except RuntimeError as exc:
        print(f"状态存储配置错误：{exc}", file=sys.stderr)
        sys.exit(2)

    now = datetime.now(TZ)

    if not (WINDOW_START <= now.time() < WINDOW_END):
        print(
            f"{now:%Y-%m-%d %H:%M} 不在 "
            f"{WINDOW_START:%H:%M}-{WINDOW_END:%H:%M} (UTC+8) 窗口内，跳过本次检查",
            file=sys.stderr,
        )
        sys.exit(0)

    try:
        result = query_visa_status(
            PASSPORT_NUMBER,
            ENGLISH_NAME,
            BIRTHDAY
        )
    except Exception as exc:
        # 网络或站点异常：不通知、不改状态，避免误报
        print(f"查询签证状态失败：{exc}", file=sys.stderr)
        sys.exit(1)

    current = state_string(result)
    try:
        prev = load_state()
    except RuntimeError as exc:
        # 存储不可用时停止本轮，避免把读取故障误判成首次运行。
        print(f"读取历史状态失败：{exc}", file=sys.stderr)
        sys.exit(1)

    if prev is None:
        # 无历史状态 → 首次运行，发一条通知并记录初始状态
        send_push(
            "签证监控已启动（首次运行）",
            format_result(result),
        )
        save_state(current, format_result(result), now)
        print(f"{now:%H:%M} 首次运行，已发送通知并记录初始状态")
        return

    if current != prev.get("status_text"):
        old_summary = prev.get("summary") or prev.get("status_text")
        send_push(
            "签证申请状态有变化",
            f"【旧状态】\n{old_summary}\n\n【新状态】\n{format_result(result)}",
        )
        save_state(current, format_result(result), now)
        print(f"{now:%H:%M} 状态变化，已发送通知")
    else:
        print(f"{now:%H:%M} 状态无变化，不通知")


if __name__ == "__main__":
    main()
