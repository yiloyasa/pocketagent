export type SceneId =
  | "signal"
  | "control-loop"
  | "phone-demo"
  | "feature-pillars"
  | "resilient-local"
  | "resolve";

export type SceneRange = {
  id: SceneId;
  start: number;
  end: number;
};

export const STORYBOARD = {
  width: 1080,
  height: 1920,
  fps: 30,
  durationInFrames: 1350,
  scenes: [
    {id: "signal", start: 0, end: 120},
    {id: "control-loop", start: 120, end: 300},
    {id: "phone-demo", start: 300, end: 660},
    {id: "feature-pillars", start: 660, end: 870},
    {id: "resilient-local", start: 870, end: 1140},
    {id: "resolve", start: 1140, end: 1350},
  ] satisfies SceneRange[],
  features: ["Chat", "Terminal", "Files", "Git", "Tasks"] as const,
};

export const scene = (id: SceneId): SceneRange =>
  STORYBOARD.scenes.find((candidate) => candidate.id === id) as SceneRange;
