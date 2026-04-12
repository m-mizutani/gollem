import { useState } from "react";
import type { LLMCallData, MessageContent } from "../api/types";
import { prettyJSON } from "../utils/format";
import MarkdownContent from "./MarkdownContent";
import JSONView from "./JSONView";

function renderMessageContent(content: MessageContent, key: number) {
  switch (content.type) {
    case "text":
      return (
        <div key={key} className="prose prose-sm max-w-none">
          <MarkdownContent content={content.text || ""} />
        </div>
      );
    case "tool_call":
      return (
        <div
          key={key}
          className="border border-purple-200 bg-purple-50 rounded p-2 text-sm"
        >
          <div className="flex items-center gap-2 mb-1">
            <span className="text-xs font-semibold text-purple-600">
              Tool Call
            </span>
            <span className="font-mono font-medium">{content.name}</span>
            {content.id && (
              <span className="text-xs text-gray-400">ID: {content.id}</span>
            )}
          </div>
          {content.arguments && (
            <JSONView data={prettyJSON(content.arguments)} />
          )}
        </div>
      );
    case "tool_response":
      return (
        <div
          key={key}
          className="border border-green-200 bg-green-50 rounded p-2 text-sm"
        >
          <div className="flex items-center gap-2 mb-1">
            <span className="text-xs font-semibold text-green-600">
              Tool Response
            </span>
            {content.name && (
              <span className="font-mono font-medium">{content.name}</span>
            )}
            {content.tool_call_id && (
              <span className="text-xs text-gray-400">
                Call ID: {content.tool_call_id}
              </span>
            )}
          </div>
          {content.result && <JSONView data={prettyJSON(content.result)} />}
        </div>
      );
    case "image":
      return (
        <div
          key={key}
          className="inline-block bg-gray-100 border border-gray-200 rounded px-2 py-1 text-xs text-gray-500"
        >
          [Image{content.media_type ? `: ${content.media_type}` : ""}]
        </div>
      );
    case "document":
    case "file":
      return (
        <div
          key={key}
          className="inline-block bg-gray-100 border border-gray-200 rounded px-2 py-1 text-xs text-gray-500"
        >
          [{content.type}
          {content.media_type ? `: ${content.media_type}` : ""}]
        </div>
      );
    case "thinking":
    case "redacted_thinking":
      return (
        <div
          key={key}
          className="inline-block bg-orange-50 border border-orange-200 rounded px-2 py-1 text-xs text-orange-500"
        >
          [{content.type}]
        </div>
      );
    default:
      return (
        <div
          key={key}
          className="inline-block bg-gray-100 rounded px-2 py-1 text-xs text-gray-500"
        >
          [{content.type}]
        </div>
      );
  }
}

interface LLMCallDetailProps {
  data: LLMCallData;
}

export default function LLMCallDetail({ data }: LLMCallDetailProps) {
  const [showSystemPrompt, setShowSystemPrompt] = useState(false);

  return (
    <div className="space-y-4">
      <div className="flex gap-4 text-sm">
        {data.model && (
          <div>
            <span className="text-gray-500">Model:</span>{" "}
            <span className="font-medium">{data.model}</span>
          </div>
        )}
        <div>
          <span className="text-gray-500">Input Tokens:</span>{" "}
          <span className="font-medium">{data.input_tokens}</span>
        </div>
        <div>
          <span className="text-gray-500">Output Tokens:</span>{" "}
          <span className="font-medium">{data.output_tokens}</span>
        </div>
      </div>

      {data.request && (
        <div className="space-y-3">
          <h4 className="font-semibold text-gray-700">Request</h4>

          {data.request.system_prompt && (
            <div className="border border-gray-200 rounded">
              <button
                onClick={() => setShowSystemPrompt(!showSystemPrompt)}
                className="w-full px-3 py-2 text-left text-sm font-medium text-gray-600 hover:bg-gray-50 flex items-center gap-1"
              >
                <span>{showSystemPrompt ? "\u25BE" : "\u25B8"}</span>
                System Prompt
              </button>
              {showSystemPrompt && (
                <div className="px-3 py-2 border-t border-gray-200 text-sm prose prose-sm max-w-none">
                  <MarkdownContent content={data.request.system_prompt} />
                </div>
              )}
            </div>
          )}

          {data.request.messages && data.request.messages.length > 0 && (
            <div>
              <h5 className="text-sm font-medium text-gray-600 mb-2">
                Messages
              </h5>
              <div className="space-y-2">
                {data.request.messages.map((msg, i) => (
                  <div
                    key={i}
                    className={`rounded p-3 text-sm ${
                      msg.role === "user"
                        ? "bg-blue-50 border border-blue-100"
                        : msg.role === "assistant"
                        ? "bg-gray-50 border border-gray-200 ml-4"
                        : msg.role === "tool"
                        ? "bg-yellow-50 border border-yellow-100"
                        : "bg-gray-50 border border-gray-200"
                    }`}
                  >
                    <div className="text-xs font-medium text-gray-500 mb-1">
                      {msg.role}
                    </div>
                    <div className="space-y-2">
                      {msg.contents?.map((content, j) =>
                        renderMessageContent(content, j)
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {data.request.tools && data.request.tools.length > 0 && (
            <div>
              <h5 className="text-sm font-medium text-gray-600 mb-2">
                Tools ({data.request.tools.length})
              </h5>
              <div className="space-y-1">
                {data.request.tools.map((tool, i) => (
                  <div
                    key={i}
                    className="px-3 py-1.5 bg-gray-50 rounded text-sm"
                  >
                    <span className="font-mono font-medium">{tool.name}</span>
                    {tool.description && (
                      <span className="text-gray-500 ml-2">
                        {tool.description}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {data.response && (
        <div className="space-y-3">
          <h4 className="font-semibold text-gray-700">Response</h4>

          {data.response.texts && data.response.texts.length > 0 && (
            <div>
              <h5 className="text-sm font-medium text-gray-600 mb-2">Texts</h5>
              <div className="space-y-2">
                {data.response.texts.map((text, i) => (
                  <div
                    key={i}
                    className="bg-gray-50 border border-gray-200 rounded p-3 text-sm prose prose-sm max-w-none"
                  >
                    <MarkdownContent content={text} />
                  </div>
                ))}
              </div>
            </div>
          )}

          {data.response.function_calls &&
            data.response.function_calls.length > 0 && (
              <div>
                <h5 className="text-sm font-medium text-gray-600 mb-2">
                  Function Calls
                </h5>
                <div className="space-y-2">
                  {data.response.function_calls.map((fc, i) => (
                    <div
                      key={i}
                      className="border border-gray-200 rounded p-3"
                    >
                      <div className="flex items-center gap-2 mb-2">
                        <span className="font-mono font-medium text-sm">
                          {fc.name}
                        </span>
                        <span className="text-xs text-gray-400">
                          ID: {fc.id}
                        </span>
                      </div>
                      {fc.arguments && (
                        <JSONView data={prettyJSON(fc.arguments)} />
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}
        </div>
      )}
    </div>
  );
}
