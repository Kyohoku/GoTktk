from schemas import VideoContext

#模型角色设定
#减少胡说八道（Hallucination）
#保守总结避免模型脑补
#明确输出格式
SYSTEM_PROMPT = """
你是一个短视频内容分析助手。
你的任务是根据提供的视频上下文，生成结构化的视频总结结果。

请严格遵守以下要求：
1. 只能根据给定的视频上下文回答，不能编造不存在的信息。  
2. 如果上下文不足，就基于已有信息做保守总结，不要虚构细节。
3. 输出必须是 JSON 对象，不能输出额外解释。
4. 字段必须包含：
   - summary
   - keywords
   - audience
   - recommend_reason
5. keywords 必须是字符串数组，建议返回 3 到 5 个关键词。
6. summary 应简洁清晰，控制在 2 到 3 句话。
7. audience 描述这个视频更适合哪些人观看。
8. recommend_reason 说明推荐观看的主要原因。
""".strip()


def build_summary_user_prompt(context: VideoContext) -> str:
    return f"""
请根据以下视频上下文生成总结。

视频ID：{context.video_id}
作者ID：{context.author_id}
作者名：{context.author_name}
标题：{context.title}
简介：{context.description}
标签：{", ".join(context.tags) if context.tags else "无"}
字幕/转写：{context.transcript if context.transcript else "无"}
评论摘要：{context.comment_summary if context.comment_summary else "无"}

请严格返回如下 JSON 格式：

{{
  "summary": "视频摘要",
  "keywords": ["关键词1", "关键词2", "关键词3"],
  "audience": "适合人群",
  "recommend_reason": "推荐理由"
}}
""".strip()
