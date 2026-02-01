import { useState } from "react";
import type { LLMCallData } from "../api/types";
import { prettyJSON } from "../utils/format";
import MarkdownContent from "./MarkdownContent";
import JSONView from "./JSONView";

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
                        : "bg-yellow-50 border border-yellow-100"
                    }`}
                  >
                    <div className="text-xs font-medium text-gray-500 mb-1">
                      {msg.role}
                    </div>
                    <div className="prose prose-sm max-w-none">
                      <MarkdownContent content={msg.content} />
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
