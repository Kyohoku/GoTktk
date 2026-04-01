from schemas import VideoContext


SYSTEM_PROMPT = """
你是一个短视频互动评论助手。
你的任务是根据提供的视频上下文，生成适合发布在该视频下的评论建议。

请严格遵守以下要求：
1. 只能基于给定的视频上下文生成评论，不能编造不存在的信息。
2. 评论要自然、简洁、像真实用户会发布的内容，不要过于机械或夸张。
3. 不要生成低质量灌水评论，例如“不错”“666”“支持一下”等过于空泛的内容。
4. 不要生成攻击性、敏感、违规或不适当内容。
5. 输出必须是 JSON 对象，不能输出额外解释。
6. 字段必须包含：
   - style
   - suggestions
7. suggestions 必须是字符串数组，返回 3 条评论建议。
8. 每条评论尽量控制在 10 到 30 个字之间。
""".strip()


def build_comment_user_prompt(context: VideoContext, style: str = "default") -> str:
    style_desc = {
        "default": "自然、真实、通用",
        "friendly": "友好、轻松、有互动感",
        "professional": "偏专业、偏内容理解型",
        "funny": "稍微轻松幽默，但不要油腻或夸张",
    }.get(style, "自然、真实、通用")

    return f"""
请根据以下视频上下文生成 3 条评论建议。

评论风格：{style}
风格说明：{style_desc}

视频ID：{context.video_id}
作者ID：{context.author_id}
作者名：{context.author_name}
标题：{context.title}
简介：{context.description}
标签：{", ".join(context.tags) if context.tags else "无"}
字幕/转写：{context.transcript if context.transcript else "无"}
评论摘要：{context.comment_summary if context.comment_summary else "无"}

请注意：
1. 评论要结合视频主题或内容细节。
2. 评论之间不要重复。
3. 评论要像真实用户发言，不要像总结报告。
4. 如果信息不足，就基于标题、简介和标签生成尽量合理的评论。

请严格返回如下 JSON 格式：

{{
  "style": "{style}",
  "suggestions": [
    "评论建议1",
    "评论建议2",
    "评论建议3"
  ]
}}
""".strip()
