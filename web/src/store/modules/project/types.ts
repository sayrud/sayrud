import {type Project} from "@/api/project.ts";

export interface ProjectState {
    currentProject: Project | null;
}
