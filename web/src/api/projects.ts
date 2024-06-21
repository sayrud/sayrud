import axios from 'axios';

export interface Project {
    uid: string;
    name: string;
    schemaName: string;
}

export interface ListProjectResp {
    projects: Project[];
    total: number;
}

export function listProjects() {
    return axios.get<ListProjectResp, ListProjectResp>('/projects');
}

export interface CreateProjectReq {
    name: string;
    schemaName: string;
}

export function createProject(data: CreateProjectReq) {
    return axios.post<Project, Project>('/projects', data);
}

export function getProject(uid: string) {
    return axios.get<Project, Project>(`/projects/${uid}`);
}

export interface UpdateProjectReq {
    name: string;
}

export function updateProject(uid: string, data: UpdateProjectReq) {
    return axios.put<void, void>(`/projects/${uid}`, data);
}

export function deleteProject(uid: string) {
    return axios.delete<void, void>(`/projects/${uid}`);
}
