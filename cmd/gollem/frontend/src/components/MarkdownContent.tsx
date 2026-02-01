import ReactMarkdown from "react-markdown";
import rehypeSanitize from "rehype-sanitize";
import { isJSON, prettyJSON } from "../utils/format";

interface MarkdownContentProps {
  content: string;
}

export default function MarkdownContent({ content }: MarkdownContentProps) {
  if (isJSON(content)) {
    return (
      <pre className="bg-gray-900 text-gray-100 rounded p-3 text-xs overflow-auto max-h-96">
        <code>{prettyJSON(content)}</code>
      </pre>
    );
  }

  return (
    <div className="prose prose-sm max-w-none prose-headings:mt-3 prose-headings:mb-1 prose-p:my-1 prose-ul:my-1 prose-ol:my-1 prose-li:my-0 prose-pre:my-2 prose-code:text-xs">
      <ReactMarkdown
        rehypePlugins={[rehypeSanitize]}
        components={{
          pre: ({ children }) => (
            <pre className="bg-gray-900 text-gray-100 rounded p-3 text-xs overflow-auto">
              {children}
            </pre>
          ),
          code: ({ children, className }) => {
            const isInline = !className;
            return isInline ? (
              <code className="bg-gray-100 text-gray-800 px-1 py-0.5 rounded text-xs">
                {children}
              </code>
            ) : (
              <code className={className}>{children}</code>
            );
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
