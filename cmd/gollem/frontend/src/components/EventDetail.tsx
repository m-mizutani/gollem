import type { EventData } from "../api/types";
import { prettyJSON } from "../utils/format";
import JSONView from "./JSONView";

interface EventDetailProps {
  data: EventData;
}

export default function EventDetail({ data }: EventDetailProps) {
  return (
    <div className="space-y-3">
      <div className="text-sm">
        <span className="text-gray-500">Event Kind:</span>{" "}
        <span className="inline-block px-1.5 py-0.5 rounded text-xs font-medium bg-orange-100 text-orange-700">
          {data.kind}
        </span>
      </div>

      <div>
        <h5 className="text-sm font-medium text-gray-600 mb-1">Data</h5>
        <JSONView data={prettyJSON(data.data)} />
      </div>
    </div>
  );
}
