// Sanjana must use the user's OpenAI/Codex subscription. A supported cloud
// execution path has not been connected; never fall back to another provider
// or separately billed API, even if old deployment variables are present.
export function providerStatus(env) {
 if(env?.RESEARCH)return '';
 return 'Sanjana uses your OpenAI/Codex subscription. Autonomous cloud research is not connected yet. Saved updates remain available.';
}
export async function researchProvider() {
 throw new Error(providerStatus());
}
