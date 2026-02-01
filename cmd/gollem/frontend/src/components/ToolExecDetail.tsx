import type { ToolExecData } from "../api/types";
import { prettyJSON } from "../utils/format";
import JSONView from "./JSONView";

interface ToolExecDetailProps {
  data: ToolExecData;
}

export default function ToolExecDetail({ data }: ToolExecDetailProps) {
  return (
    <div className="space-y-3">
      <div className="text-sm">
        <span className="text-gray-500">Tool:</span>{" "}
        <span className="font-mono font-medium">{data.tool_name}</span>
      </div>

      <div>
        <h5 className="text-sm font-medium text-gray-600 mb-1">Arguments</h5>
        <JSONView data={prettyJSON(data.args)} />
      </div>

      {data.result && (
        <div>
          <h5 className="text-sm font-medium text-gray-600 mb-1">Result</h5>
          <JSONView data={prettyJSON(data.result)} />
        </div>
      )}

      {data.error && (
        <div className="bg-red-50 border border-red-200 rounded p-3 text-sm text-red-700">
          <span className="font-medium">Error:</span> {data.error}
        </div>
      )}
    </div>
  );
}
