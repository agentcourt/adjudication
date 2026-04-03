#!/usr/bin/env node

const fs = require("fs");

const path =
  "/usr/local/lib/node_modules/@mariozechner/pi-coding-agent/node_modules/@mariozechner/pi-ai/dist/providers/openai-responses.js";
const sharedPath =
  "/usr/local/lib/node_modules/@mariozechner/pi-coding-agent/node_modules/@mariozechner/pi-ai/dist/providers/openai-responses-shared.js";

const src = fs.readFileSync(path, "utf8");

const modelLineNeedle = `    const params = {\n        model: model.id,`;
const modelLineReplace = `    const responsesCompat = model.compat;\n    const requestModel = responsesCompat?.modelId ?? model.id;\n    const params = {\n        model: requestModel,`;

if (!src.includes(modelLineNeedle)) {
  throw new Error("Could not find model assignment in openai-responses.js");
}

const toolsNeedle = `    if (context.tools) {\n        params.tools = convertResponsesTools(context.tools);\n    }\n`;
const toolsReplace = `    if (context.tools) {\n        params.tools = convertResponsesTools(context.tools);\n    }\n    if (responsesCompat?.webSearchEnabled) {\n        const webSearchOptions = responsesCompat.webSearchOptions ?? {};\n        const webSearchTool = { type: "web_search", ...webSearchOptions };\n        if (params.tools) {\n            params.tools.push(webSearchTool);\n        }\n        else {\n            params.tools = [webSearchTool];\n        }\n        if (responsesCompat.toolChoice === "required") {\n            params.tool_choice = "required";\n        }\n    }\n    if (typeof requestModel === "string" && requestModel.includes("deep-research")) {\n        const deepResearchTools = responsesCompat?.deepResearchTools ?? [\n            { type: "web_search_preview" },\n            { type: "code_interpreter" },\n            { type: "mcp" },\n            { type: "file_search" },\n        ];\n        params.tools = deepResearchTools;\n        params.tool_choice = "auto";\n    }\n`;

if (!src.includes(toolsNeedle)) {
  throw new Error("Could not find tools block in openai-responses.js");
}

let out = src.replace(modelLineNeedle, modelLineReplace);
out = out.replace(toolsNeedle, toolsReplace);

fs.writeFileSync(path, out);

const sharedSrc = fs.readFileSync(sharedPath, "utf8");
const reasoningNeedle = `                if (block.thinkingSignature) {\n                        const reasoningItem = JSON.parse(block.thinkingSignature);\n                        output.push(reasoningItem);\n                    }\n`;
const reasoningReplace = `                if (false && block.thinkingSignature) {\n                        const reasoningItem = JSON.parse(block.thinkingSignature);\n                        output.push(reasoningItem);\n                    }\n`;

if (!sharedSrc.includes(reasoningNeedle)) {
  throw new Error("Could not find reasoning replay block in openai-responses-shared.js");
}

const sharedOut = sharedSrc.replace(reasoningNeedle, reasoningReplace);
fs.writeFileSync(sharedPath, sharedOut);
