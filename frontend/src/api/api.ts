/* eslint-disable */
/* tslint:disable */
// @ts-nocheck
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */

export interface AIAdvice {
  action: "tables";
  messages: AIMessage[];
}

export interface AIAdviceResp {
  action: string;
  /** ActionJSON is the structured payload of the action, which can be passed to the apply API as is. */
  actionJson: object;
  description: string;
  /** Raw is the raw output of the model. */
  raw: string;
}

export interface AIApply {
  action: "tables";
  /** ActionJson is the actionJson returned by the advice API. */
  actionJson: object;
}

export interface AIMessage {
  content: string;
  role: string;
}

export interface BatchCreateRecords {
  /** Data is the list of cell values keyed by field UID, one item for each record. */
  data: Record<string, any>[];
}

export interface CreateField {
  label: string;
  /** Metadata is the type-specific configuration, e.g. select options or the formula expression. */
  metadata?: Record<string, any>;
  type:
    | "text"
    | "single_select"
    | "multi_select"
    | "datetime"
    | "number"
    | "checkbox"
    | "formula";
}

export interface CreateFields {
  fields: CreateField[];
}

export interface CreateProject {
  name: string;
  /** SchemaName is the Postgres schema of the project, a random one is generated if empty. */
  schemaName?: string;
}

export interface CreateRecord {
  /** Data is the cell values keyed by field UID. */
  data?: Record<string, any>;
}

export interface CreateTable {
  name: string;
}

export interface ListProjectsResp {
  projects: ProjectListItem[];
  total: number;
}

export interface ListRecordsResp {
  records: SLRecord[];
  /** Total is the number of records matching the query, regardless of limit and offset. */
  total: number;
}

export interface ListTablesResp {
  tables: TableListItem[];
  total: number;
}

export interface Profile {
  email: string;
  emailMd5: string;
  userName: string;
}

export interface Project {
  createdAt: string;
  name: string;
  schemaName: string;
  uid: string;
  updatedAt: string;
}

export interface ProjectListItem {
  createdAt: string;
  name: string;
  schemaName: string;
  tableCount: number;
  uid: string;
  updatedAt: string;
}

export interface QueryRecords {
  filter?: QueryRecordsFilter[];
  /** Group sorts the records by the field values before Order, so records in the same group are adjacent. */
  group?: QueryRecordsGroup[];
  /** Limit defaults to 20 when it is not positive. */
  limit?: number;
  offset?: number;
  order?: QueryRecordsSort[];
}

export interface QueryRecordsFilter {
  fieldUID: string;
  operation: "eq" | "neq" | "gt" | "lt" | "gte" | "lte" | "in" | "nin" | "like";
  /** Value is a JSON array (e.g. `["a","b"]`) for in / nin, and a single value for other operations. */
  value: string;
}

export interface QueryRecordsGroup {
  fieldUID: string;
}

export interface QueryRecordsSort {
  fieldUID: string;
  order?: "asc" | "desc";
}

export interface SLField {
  createdAt: string;
  label: string;
  /** Metadata is the type-specific configuration, e.g. select options or the formula expression. */
  metadata: Record<string, any>;
  position: number;
  tableUID: string;
  type:
    | "text"
    | "single_select"
    | "multi_select"
    | "datetime"
    | "number"
    | "checkbox"
    | "formula";
  uid: string;
  updatedAt: string;
}

export interface SLRecord {
  createdAt: string;
  /** Data is the cell values keyed by field UID. */
  data: Record<string, any>;
  tableUID: string;
  uid: string;
  updatedAt: string;
}

export interface SLTable {
  createdAt: string;
  name: string;
  projectUID: string;
  uid: string;
  updatedAt: string;
}

export interface TableListItem {
  count: number;
  createdAt: string;
  name: string;
  projectUID: string;
  uid: string;
  updatedAt: string;
}

export interface UpdateField {
  label?: string;
  /** Metadata replaces the whole field configuration, it is required when the type changes. */
  metadata?: Record<string, any>;
  type?:
    | "text"
    | "single_select"
    | "multi_select"
    | "datetime"
    | "number"
    | "checkbox"
    | "formula";
}

export interface UpdateFieldPosition {
  /** Position is the new position of the field, negative values are treated as 0. */
  position: number;
}

export interface UpdateProject {
  name: string;
}

export interface UpdateRecord {
  /** Data replaces all the cell values of the record, keyed by field UID. */
  data: Record<string, any>;
}

export interface UpdateTable {
  name: string;
}

export type GetProfileData = Profile;

export type ListProjectsData = ListProjectsResp;

export type CreateProjectData = Project;

export type GetProjectData = Project;

export type UpdateProjectData = any;

export type DeleteProjectData = any;

export type AiAdviceData = AIAdviceResp;

export type AiApplyData = any;

export type ListTablesData = ListTablesResp;

export type CreateTableData = SLTable;

export type ListFieldTypesData = Record<string, string>;

export type GetTableData = SLTable;

export type UpdateTableData = any;

export type DeleteTableData = any;

export type ListFieldsData = SLField[];

export type CreateFieldsData = SLField[];

export type UpdateFieldData = SLField;

export type DeleteFieldData = any;

export type UpdateFieldPositionData = any;

export type ListRecordsData = ListRecordsResp;

export type CreateRecordData = SLRecord;

export type BatchCreateRecordsData = SLRecord[];

export type QueryRecordsData = ListRecordsResp;

export type GetRecordData = SLRecord;

export type UpdateRecordData = any;

export type DeleteRecordData = any;

import type {
  AxiosInstance,
  AxiosRequestConfig,
  AxiosResponse,
  HeadersDefaults,
  ResponseType,
} from "axios";
import axios from "axios";

export type QueryParamsType = Record<string | number, any>;

export interface FullRequestParams
  extends Omit<AxiosRequestConfig, "data" | "params" | "url" | "responseType"> {
  /** set parameter to `true` for call `securityWorker` for this request */
  secure?: boolean;
  /** request path */
  path: string;
  /** content type of request body */
  type?: ContentType;
  /** query params */
  query?: QueryParamsType;
  /** format of response (i.e. response.json() -> format: "json") */
  format?: ResponseType;
  /** request body */
  body?: unknown;
}

export type RequestParams = Omit<
  FullRequestParams,
  "body" | "method" | "query" | "path"
>;

export interface ApiConfig<SecurityDataType = unknown>
  extends Omit<AxiosRequestConfig, "data" | "cancelToken"> {
  securityWorker?: (
    securityData: SecurityDataType | null,
  ) => Promise<AxiosRequestConfig | void> | AxiosRequestConfig | void;
  secure?: boolean;
  format?: ResponseType;
}

export enum ContentType {
  Json = "application/json",
  JsonApi = "application/vnd.api+json",
  FormData = "multipart/form-data",
  UrlEncoded = "application/x-www-form-urlencoded",
  Text = "text/plain",
}

export class HttpClient<SecurityDataType = unknown> {
  public instance: AxiosInstance;
  private securityData: SecurityDataType | null = null;
  private securityWorker?: ApiConfig<SecurityDataType>["securityWorker"];
  private secure?: boolean;
  private format?: ResponseType;

  constructor({
    securityWorker,
    secure,
    format,
    ...axiosConfig
  }: ApiConfig<SecurityDataType> = {}) {
    this.instance = axios.create({
      ...axiosConfig,
      baseURL: axiosConfig.baseURL || "/_",
    });
    this.secure = secure;
    this.format = format;
    this.securityWorker = securityWorker;
  }

  public setSecurityData = (data: SecurityDataType | null) => {
    this.securityData = data;
  };

  protected mergeRequestParams(
    params1: AxiosRequestConfig,
    params2?: AxiosRequestConfig,
  ): AxiosRequestConfig {
    const method = params1.method || (params2 && params2.method);

    return {
      ...this.instance.defaults,
      ...params1,
      ...(params2 || {}),
      headers: {
        ...((method &&
          this.instance.defaults.headers[
            method.toLowerCase() as keyof HeadersDefaults
          ]) ||
          {}),
        ...(params1.headers || {}),
        ...((params2 && params2.headers) || {}),
      },
    };
  }

  protected stringifyFormItem(formItem: unknown) {
    if (typeof formItem === "object" && formItem !== null) {
      return JSON.stringify(formItem);
    } else {
      return `${formItem}`;
    }
  }

  protected createFormData(input: Record<string, unknown>): FormData {
    if (input instanceof FormData) {
      return input;
    }
    return Object.keys(input || {}).reduce((formData, key) => {
      const property = input[key];
      const propertyContent: any[] =
        property instanceof Array ? property : [property];

      for (const formItem of propertyContent) {
        const isFileType = formItem instanceof Blob || formItem instanceof File;
        formData.append(
          key,
          isFileType ? formItem : this.stringifyFormItem(formItem),
        );
      }

      return formData;
    }, new FormData());
  }

  public request = async <T = any, _E = any>({
    secure,
    path,
    type,
    query,
    format,
    body,
    ...params
  }: FullRequestParams): Promise<AxiosResponse<T>> => {
    const secureParams =
      ((typeof secure === "boolean" ? secure : this.secure) &&
        this.securityWorker &&
        (await this.securityWorker(this.securityData))) ||
      {};
    const requestParams = this.mergeRequestParams(params, secureParams);
    const responseFormat = format || this.format || undefined;

    if (
      type === ContentType.FormData &&
      body &&
      body !== null &&
      typeof body === "object"
    ) {
      body = this.createFormData(body as Record<string, unknown>);
    }

    if (
      type === ContentType.Text &&
      body &&
      body !== null &&
      typeof body !== "string"
    ) {
      body = JSON.stringify(body);
    }

    return this.instance.request({
      ...requestParams,
      headers: {
        ...(requestParams.headers || {}),
        ...(type ? { "Content-Type": type } : {}),
      },
      params: query,
      responseType: responseFormat,
      data: body,
      url: path,
    });
  };
}

/**
 * @title Sayrud API
 * @version 1.0
 * @baseUrl /_
 * @contact
 */
export class Api<
  SecurityDataType extends unknown,
> extends HttpClient<SecurityDataType> {
  auth = {
    /**
     * No description
     *
     * @name GetProfile
     * @summary Get the profile of the signed-in user
     * @request GET:/auth/profile
     */
    getProfile: (params: RequestParams = {}) =>
      this.request<GetProfileData, string>({
        path: `/auth/profile`,
        method: "GET",
        format: "json",
        ...params,
      }),
  };
  projects = {
    /**
     * @description List the projects owned by the signed-in user, with the number of tables in each project.
     *
     * @name ListProjects
     * @summary List projects
     * @request GET:/projects
     */
    listProjects: (
      query?: {
        /** Page number, starting from 1 */
        page?: number;
        /** Page size, defaults to 20 */
        pageSize?: number;
      },
      params: RequestParams = {},
    ) =>
      this.request<ListProjectsData, string>({
        path: `/projects`,
        method: "GET",
        query: query,
        format: "json",
        ...params,
      }),

    /**
     * @description Create a project owned by the signed-in user, along with its Postgres schema.
     *
     * @name CreateProject
     * @summary Create a project
     * @request POST:/projects
     */
    createProject: (data: CreateProject, params: RequestParams = {}) =>
      this.request<CreateProjectData, string>({
        path: `/projects`,
        method: "POST",
        body: data,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @name GetProject
     * @summary Get a project
     * @request GET:/projects/{projectUID}
     */
    getProject: (projectUid: string, params: RequestParams = {}) =>
      this.request<GetProjectData, string>({
        path: `/projects/${projectUid}`,
        method: "GET",
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @name UpdateProject
     * @summary Update a project
     * @request PUT:/projects/{projectUID}
     */
    updateProject: (
      projectUid: string,
      data: UpdateProject,
      params: RequestParams = {},
    ) =>
      this.request<UpdateProjectData, string>({
        path: `/projects/${projectUid}`,
        method: "PUT",
        body: data,
        type: ContentType.Json,
        ...params,
      }),

    /**
     * @description Delete the project and drop its Postgres schema.
     *
     * @name DeleteProject
     * @summary Delete a project
     * @request DELETE:/projects/{projectUID}
     */
    deleteProject: (projectUid: string, params: RequestParams = {}) =>
      this.request<DeleteProjectData, string>({
        path: `/projects/${projectUid}`,
        method: "DELETE",
        ...params,
      }),

    /**
     * @description Generate the advice of the action by AI from the conversation messages.
     *
     * @name AiAdvice
     * @summary Get table design advice from AI
     * @request POST:/projects/{projectUID}/ai/advice
     */
    aiAdvice: (
      projectUid: string,
      data: AIAdvice,
      params: RequestParams = {},
    ) =>
      this.request<AiAdviceData, string>({
        path: `/projects/${projectUid}/ai/advice`,
        method: "POST",
        body: data,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * @description Apply the actionJson returned by the advice API, e.g. create the advised tables.
     *
     * @name AiApply
     * @summary Apply the AI advice
     * @request POST:/projects/{projectUID}/ai/apply
     */
    aiApply: (projectUid: string, data: AIApply, params: RequestParams = {}) =>
      this.request<AiApplyData, string>({
        path: `/projects/${projectUid}/ai/apply`,
        method: "POST",
        body: data,
        type: ContentType.Json,
        ...params,
      }),

    /**
     * @description List the tables of the project, with the number of records in each table.
     *
     * @name ListTables
     * @summary List tables
     * @request GET:/projects/{projectUID}/tables
     */
    listTables: (
      projectUid: string,
      query?: {
        /** Page number, starting from 1 */
        page?: number;
        /** Page size, defaults to 20 */
        pageSize?: number;
      },
      params: RequestParams = {},
    ) =>
      this.request<ListTablesData, string>({
        path: `/projects/${projectUid}/tables`,
        method: "GET",
        query: query,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @name CreateTable
     * @summary Create a table
     * @request POST:/projects/{projectUID}/tables
     */
    createTable: (
      projectUid: string,
      data: CreateTable,
      params: RequestParams = {},
    ) =>
      this.request<CreateTableData, string>({
        path: `/projects/${projectUid}/tables`,
        method: "POST",
        body: data,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * @description Return the labels of all the field types, keyed by field type.
     *
     * @name ListFieldTypes
     * @summary List field types
     * @request GET:/projects/{projectUID}/tables/types
     */
    listFieldTypes: (projectUid: string, params: RequestParams = {}) =>
      this.request<ListFieldTypesData, string>({
        path: `/projects/${projectUid}/tables/types`,
        method: "GET",
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @name GetTable
     * @summary Get a table
     * @request GET:/projects/{projectUID}/tables/{tableUID}
     */
    getTable: (
      projectUid: string,
      tableUid: string,
      params: RequestParams = {},
    ) =>
      this.request<GetTableData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}`,
        method: "GET",
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @name UpdateTable
     * @summary Update a table
     * @request PUT:/projects/{projectUID}/tables/{tableUID}
     */
    updateTable: (
      projectUid: string,
      tableUid: string,
      data: UpdateTable,
      params: RequestParams = {},
    ) =>
      this.request<UpdateTableData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}`,
        method: "PUT",
        body: data,
        type: ContentType.Json,
        ...params,
      }),

    /**
     * No description
     *
     * @name DeleteTable
     * @summary Delete a table
     * @request DELETE:/projects/{projectUID}/tables/{tableUID}
     */
    deleteTable: (
      projectUid: string,
      tableUid: string,
      params: RequestParams = {},
    ) =>
      this.request<DeleteTableData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}`,
        method: "DELETE",
        ...params,
      }),

    /**
     * @description List all the fields of the table, ordered by position.
     *
     * @name ListFields
     * @summary List fields
     * @request GET:/projects/{projectUID}/tables/{tableUID}/fields
     */
    listFields: (
      projectUid: string,
      tableUid: string,
      params: RequestParams = {},
    ) =>
      this.request<ListFieldsData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/fields`,
        method: "GET",
        format: "json",
        ...params,
      }),

    /**
     * @description Create fields in batch, which are appended to the end of the table.
     *
     * @name CreateFields
     * @summary Create fields
     * @request POST:/projects/{projectUID}/tables/{tableUID}/fields
     */
    createFields: (
      projectUid: string,
      tableUid: string,
      data: CreateFields,
      params: RequestParams = {},
    ) =>
      this.request<CreateFieldsData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/fields`,
        method: "POST",
        body: data,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * @description Update the label, type or metadata of the field. Existing record values are kept as is when the type changes.
     *
     * @name UpdateField
     * @summary Update a field
     * @request PUT:/projects/{projectUID}/tables/{tableUID}/fields/{fieldUID}
     */
    updateField: (
      projectUid: string,
      tableUid: string,
      fieldUid: string,
      data: UpdateField,
      params: RequestParams = {},
    ) =>
      this.request<UpdateFieldData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/fields/${fieldUid}`,
        method: "PUT",
        body: data,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @name DeleteField
     * @summary Delete a field
     * @request DELETE:/projects/{projectUID}/tables/{tableUID}/fields/{fieldUID}
     */
    deleteField: (
      projectUid: string,
      tableUid: string,
      fieldUid: string,
      params: RequestParams = {},
    ) =>
      this.request<DeleteFieldData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/fields/${fieldUid}`,
        method: "DELETE",
        ...params,
      }),

    /**
     * @description Set the position of the field, the fields at or after the position are moved back by one.
     *
     * @name UpdateFieldPosition
     * @summary Move a field
     * @request PUT:/projects/{projectUID}/tables/{tableUID}/fields/{fieldUID}/position
     */
    updateFieldPosition: (
      projectUid: string,
      tableUid: string,
      fieldUid: string,
      data: UpdateFieldPosition,
      params: RequestParams = {},
    ) =>
      this.request<UpdateFieldPositionData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/fields/${fieldUid}/position`,
        method: "PUT",
        body: data,
        type: ContentType.Json,
        ...params,
      }),

    /**
     * @description List the records of the table, the newest first.
     *
     * @name ListRecords
     * @summary List records
     * @request GET:/projects/{projectUID}/tables/{tableUID}/records
     */
    listRecords: (
      projectUid: string,
      tableUid: string,
      query?: {
        /** Maximum number of records, defaults to 20 */
        limit?: number;
        /** Number of records to skip */
        offset?: number;
      },
      params: RequestParams = {},
    ) =>
      this.request<ListRecordsData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/records`,
        method: "GET",
        query: query,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @name CreateRecord
     * @summary Create a record
     * @request POST:/projects/{projectUID}/tables/{tableUID}/records
     */
    createRecord: (
      projectUid: string,
      tableUid: string,
      data: CreateRecord,
      params: RequestParams = {},
    ) =>
      this.request<CreateRecordData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/records`,
        method: "POST",
        body: data,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * @description Create multiple records atomically, none is created if any data is invalid.
     *
     * @name BatchCreateRecords
     * @summary Create records in batch
     * @request POST:/projects/{projectUID}/tables/{tableUID}/records/batch
     */
    batchCreateRecords: (
      projectUid: string,
      tableUid: string,
      data: BatchCreateRecords,
      params: RequestParams = {},
    ) =>
      this.request<BatchCreateRecordsData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/records/batch`,
        method: "POST",
        body: data,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * @description Filter, group and sort the records of the table. Formula fields cannot be queried.
     *
     * @name QueryRecords
     * @summary Query records
     * @request POST:/projects/{projectUID}/tables/{tableUID}/records/query
     */
    queryRecords: (
      projectUid: string,
      tableUid: string,
      data: QueryRecords,
      params: RequestParams = {},
    ) =>
      this.request<QueryRecordsData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/records/query`,
        method: "POST",
        body: data,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @name GetRecord
     * @summary Get a record
     * @request GET:/projects/{projectUID}/tables/{tableUID}/records/{recordUID}
     */
    getRecord: (
      projectUid: string,
      tableUid: string,
      recordUid: string,
      params: RequestParams = {},
    ) =>
      this.request<GetRecordData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/records/${recordUid}`,
        method: "GET",
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @name UpdateRecord
     * @summary Update a record
     * @request PUT:/projects/{projectUID}/tables/{tableUID}/records/{recordUID}
     */
    updateRecord: (
      projectUid: string,
      tableUid: string,
      recordUid: string,
      data: UpdateRecord,
      params: RequestParams = {},
    ) =>
      this.request<UpdateRecordData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/records/${recordUid}`,
        method: "PUT",
        body: data,
        type: ContentType.Json,
        ...params,
      }),

    /**
     * No description
     *
     * @name DeleteRecord
     * @summary Delete a record
     * @request DELETE:/projects/{projectUID}/tables/{tableUID}/records/{recordUID}
     */
    deleteRecord: (
      projectUid: string,
      tableUid: string,
      recordUid: string,
      params: RequestParams = {},
    ) =>
      this.request<DeleteRecordData, string>({
        path: `/projects/${projectUid}/tables/${tableUid}/records/${recordUid}`,
        method: "DELETE",
        ...params,
      }),
  };
}
