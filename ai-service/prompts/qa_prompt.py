from schemas import VideoContext


SYSTEM_PROMPT = """
你是一个短视频问答助手。
你的任务是根据提供的视频上下文，回答用户关于当前视频的问题。

请严格遵守以下要求：
1. 只能根据给定的视频上下文回答，不能编造信息。
2. 只回答和当前视频内容相关的问题。
3. 如果问题超出当前视频范围，必须明确说明“我只能回答和当前视频相关的问题”。
4. 如果上下文不足以支持回答，必须明确说明“根据当前视频提供的信息，暂时无法准确回答这个问题”。
5. 回答要简洁、直接、自然，不要写成报告或长篇分析。
6. 输出必须是 JSON 对象，不能输出额外解释。
7. 字段必须包含：
   - answer
""".strip()


def build_qa_user_prompt(context: VideoContext, question: str) -> str:
    return f"""
请根据以下视频上下文，回答用户关于当前视频的问题。

用户问题：{question}

视频ID：{context.video_id}
作者ID：{context.author_id}
作者名：{context.author_name}
标题：{context.title}
简介：{context.description}
标签：{", ".join(context.tags) if context.tags else "无"}
字幕/转写：{context.transcript if context.transcript else "无"}
评论摘要：{context.comment_summary if context.comment_summary else "无"}

请注意：
1. 你的回答必须围绕当前视频内容展开。
2. 不要回答与当前视频无关的问题。
3. 不要猜测视频中没有提到的信息。
4. 如果问题超出视频范围，请直接拒答。
5. 如果信息不足，请明确说信息不足。

请严格返回如下 JSON 格式：

{{
  "answer": "你的回答"
}}
""".strip()
