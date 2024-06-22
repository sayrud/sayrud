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

export function listProjects(params: { page?: number, pageSize?: number }) {
    return axios.get<ListProjectResp, ListProjectResp>('/projects', {params});
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
