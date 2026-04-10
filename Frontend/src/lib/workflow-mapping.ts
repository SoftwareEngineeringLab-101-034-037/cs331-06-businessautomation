import type { ConditionDataType, TriggerFieldSchemaItem } from "@/types/workflow";

export function parseFieldMapping(raw: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const pair of raw.split(",")) {
    const [source, target] = pair.split(":").map((v) => v.trim());
    if (source && target) out[source] = target;
  }
  return out;
}

export function serializeFieldMapping(mapping: Record<string, string>): string {
  return Object.entries(mapping)
    .filter(([source, target]) => source.trim() && target.trim())
    .map(([source, target]) => `${source}:${target}`)
    .join(", ");
}

export function normalizeConditionDataType(raw: string | undefined | null): ConditionDataType {
  const value = String(raw || "").trim().toLowerCase();
  if (value === "number" || value === "numeric" || value === "int" || value === "float") return "number";
  if (value === "boolean" || value === "bool") return "boolean";
  if (value === "date") return "date";
  if (value === "datetime" || value === "date_time" || value === "timestamp") return "datetime";
  if (value === "time") return "time";
  return "text";
}

export function inferConditionDataTypeFromFormFieldType(fieldType: string | undefined | null): ConditionDataType {
  const value = String(fieldType || "").trim().toLowerCase();
  switch (value) {
    case "scale":
      return "number";
    case "date":
      return "date";
    case "time":
      return "time";
    case "email":
    case "paragraph":
    case "choice":
    case "checkbox":
    case "dropdown":
    case "file":
    case "text":
    default:
      return "text";
  }
}

export function parseFieldSchema(raw: string): TriggerFieldSchemaItem[] {
  if (!raw.trim()) return [];
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];

    const out: TriggerFieldSchemaItem[] = [];
    for (const item of parsed) {
      if (!item || typeof item !== "object") continue;
      const questionID = String((item as any).question_id || "").trim();
      if (!questionID) continue;
      const rawDataType = String((item as any).data_type || "").trim();
      out.push({
        question_id: questionID,
        title: String((item as any).title || "").trim() || questionID,
        required: Boolean((item as any).required),
        field_type: String((item as any).field_type || "").trim(),
        variable: String((item as any).variable || "").trim(),
        data_type: rawDataType ? normalizeConditionDataType(rawDataType) : undefined,
      });
    }
    return out;
  } catch {
    return [];
  }
}

export function buildFieldSchemaJSON(
  fields: Array<{
    question_id: string;
    title: string;
    required?: boolean;
    field_type?: string;
  }>,
  mapping: Record<string, string>,
  options?: {
    dataTypeOverrides?: Record<string, ConditionDataType>;
    existingSchemaRaw?: string;
  },
): string {
  const overrides = options?.dataTypeOverrides || {};
  const existingItems = parseFieldSchema(options?.existingSchemaRaw || "");
  const existingByQuestionID = new Map(existingItems.map((item) => [item.question_id, item]));

  return JSON.stringify(
    fields.map((field) => {
      const existing = existingByQuestionID.get(field.question_id);
      const fallbackType = inferConditionDataTypeFromFormFieldType(field.field_type || existing?.field_type);
      const existingType = existing?.data_type ? normalizeConditionDataType(existing.data_type) : undefined;
      const explicitType = overrides[field.question_id] || existingType;
      return {
        question_id: field.question_id,
        title: field.title,
        required: Boolean(field.required),
        field_type: field.field_type || existing?.field_type || "text",
        variable: (mapping[field.question_id] || existing?.variable || "").trim(),
        data_type: explicitType || fallbackType,
      };
    }),
  );
}
