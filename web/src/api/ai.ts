import axios from "axios";

export interface AiDialogMessage {
    role: string;
    content: string;
}

export interface AiAdviceReq {
    action: string;
    messages: AiDialogMessage[];
}

export interface AiAdviceResp {
    raw: string;
    action: string;
    actionJson: any;
    description: string;
}

export function aiAdvice(projectUID: string, data: AiAdviceReq) {
    return axios.post<AiAdviceResp, AiAdviceResp>(`/projects/${projectUID}/ai/advice`, data);
}

export interface AiApplyReq {
    action: string;
    actionJson: any;
}

export function aiApply(projectUID: string, data: AiApplyReq) {
    return axios.post(`/projects/${projectUID}/ai/apply`, data);
}
