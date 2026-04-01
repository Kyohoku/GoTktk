from typing import List, Optional

from pydantic import BaseModel, Field

# 明确 Python AI 子服务的输入输出结构

#视频上下文
class VideoContext(BaseModel):
    video_id: int
    author_id: int
    author_name: str
    title: str
    description: str = ""
    tags: List[str] = Field(default_factory=list)
    transcript: str = ""
    comment_summary: str = ""
    source_text: str = ""  #拼装好的文本上下文
    source_text_hash: str = ""

#总结接口的输入和输出
class SummaryRequest(BaseModel):
    context: VideoContext


class SummaryResponse(BaseModel):
    summary: str
    keywords: List[str]
    audience: str
    recommend_reason: str


class QARequest(BaseModel):
    context: VideoContext
    question: str


class QAResponse(BaseModel):
    answer: str


class CommentSuggestionRequest(BaseModel):
    context: VideoContext
    style: str = "default"


class CommentSuggestionResponse(BaseModel):
    style: str
    suggestions: List[str]


class ErrorResponse(BaseModel):
    message: str
    detail: Optional[str] = None
