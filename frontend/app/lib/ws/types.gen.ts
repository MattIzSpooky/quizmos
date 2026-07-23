// To parse this data:
//
//   import { Convert, AnswerCount, AnswerResult, AnswerSubmit, ErrorPayload, GameEnded, GameStarted, LeaderboardEntry, LeaderboardUpdated, PlayerKicked, PlayerSummary, PresencePlayerJoined, PresencePlayerLeft, QuestionEnded, QuestionOption, QuestionReviewed, QuestionStarted } from "./file";
//
//   const answerCount = Convert.toAnswerCount(json);
//   const answerResult = Convert.toAnswerResult(json);
//   const answerSubmit = Convert.toAnswerSubmit(json);
//   const errorPayload = Convert.toErrorPayload(json);
//   const gameEnded = Convert.toGameEnded(json);
//   const gameStarted = Convert.toGameStarted(json);
//   const leaderboardEntry = Convert.toLeaderboardEntry(json);
//   const leaderboardUpdated = Convert.toLeaderboardUpdated(json);
//   const playerKicked = Convert.toPlayerKicked(json);
//   const playerSummary = Convert.toPlayerSummary(json);
//   const presencePlayerJoined = Convert.toPresencePlayerJoined(json);
//   const presencePlayerLeft = Convert.toPresencePlayerLeft(json);
//   const questionEnded = Convert.toQuestionEnded(json);
//   const questionOption = Convert.toQuestionOption(json);
//   const questionReviewed = Convert.toQuestionReviewed(json);
//   const questionStarted = Convert.toQuestionStarted(json);
//
// These functions will throw an error if the JSON doesn't
// match the expected interface, even if the JSON is valid.

export interface AnswerResult {
    correct:       boolean;
    pointsAwarded: number;
    questionId:    string;
    totalScore:    number;
    [property: string]: any;
}

export interface AnswerSubmit {
    optionId:   string;
    questionId: string;
    [property: string]: any;
}

export interface ErrorPayload {
    code:    string;
    message: string;
    [property: string]: any;
}

export interface GameEnded {
    endedAt:          Date;
    finalLeaderboard: LeaderboardEntry[];
    [property: string]: any;
}

export interface LeaderboardEntry {
    clientId: string;
    nickname: string;
    rank:     number;
    score:    number;
    [property: string]: any;
}

export interface GameStarted {
    startedAt: Date;
    [property: string]: any;
}

export interface LeaderboardUpdated {
    entries:       LeaderboardEntry[];
    questionIndex: number;
    [property: string]: any;
}

/**
 * Unicast to the removed player only; the rest of the room instead sees the normal
 * presence.playerLeft once the connection drops.
 */
export interface PlayerKicked {
    reason: string;
    [property: string]: any;
}

export interface PresencePlayerJoined {
    player:      PlayerSummary;
    playerCount: number;
    [property: string]: any;
}

export interface PlayerSummary {
    clientId: string;
    nickname: string;
    [property: string]: any;
}

export interface PresencePlayerLeft {
    clientId:    string;
    playerCount: number;
    [property: string]: any;
}

export interface QuestionEnded {
    answerCounts:    AnswerCount[];
    correctOptionId: string;
    questionId:      string;
    questionIndex:   number;
    [property: string]: any;
}

export interface AnswerCount {
    count:    number;
    optionId: string;
    [property: string]: any;
}

/**
 * A read-only recap of a previous question, sent when the admin goes back. Unlike
 * question.started, the correct answer is already known (it was already revealed), and
 * clients must not offer a way to answer it.
 */
export interface QuestionReviewed {
    answerCounts:    AnswerCount[];
    correctOptionId: string;
    options:         QuestionOption[];
    prompt:          string;
    questionId:      string;
    questionIndex:   number;
    totalQuestions:  number;
    [property: string]: any;
}

export interface QuestionOption {
    id:   string;
    text: string;
    [property: string]: any;
}

export interface QuestionStarted {
    options:       QuestionOption[];
    prompt:        string;
    questionId:    string;
    questionIndex: number;
    /**
     * Whether the client should show a countdown for this question.
     */
    timed:            boolean;
    timeLimitSeconds: number;
    totalQuestions:   number;
    [property: string]: any;
}

// Converts JSON strings to/from your types
// and asserts the results of JSON.parse at runtime
export class Convert {
    public static toAnswerCount(json: string): AnswerCount {
        return cast(JSON.parse(json), r("AnswerCount"));
    }

    public static answerCountToJson(value: AnswerCount): string {
        return JSON.stringify(uncast(value, r("AnswerCount")), null, 2);
    }

    public static toAnswerResult(json: string): AnswerResult {
        return cast(JSON.parse(json), r("AnswerResult"));
    }

    public static answerResultToJson(value: AnswerResult): string {
        return JSON.stringify(uncast(value, r("AnswerResult")), null, 2);
    }

    public static toAnswerSubmit(json: string): AnswerSubmit {
        return cast(JSON.parse(json), r("AnswerSubmit"));
    }

    public static answerSubmitToJson(value: AnswerSubmit): string {
        return JSON.stringify(uncast(value, r("AnswerSubmit")), null, 2);
    }

    public static toErrorPayload(json: string): ErrorPayload {
        return cast(JSON.parse(json), r("ErrorPayload"));
    }

    public static errorPayloadToJson(value: ErrorPayload): string {
        return JSON.stringify(uncast(value, r("ErrorPayload")), null, 2);
    }

    public static toGameEnded(json: string): GameEnded {
        return cast(JSON.parse(json), r("GameEnded"));
    }

    public static gameEndedToJson(value: GameEnded): string {
        return JSON.stringify(uncast(value, r("GameEnded")), null, 2);
    }

    public static toGameStarted(json: string): GameStarted {
        return cast(JSON.parse(json), r("GameStarted"));
    }

    public static gameStartedToJson(value: GameStarted): string {
        return JSON.stringify(uncast(value, r("GameStarted")), null, 2);
    }

    public static toLeaderboardEntry(json: string): LeaderboardEntry {
        return cast(JSON.parse(json), r("LeaderboardEntry"));
    }

    public static leaderboardEntryToJson(value: LeaderboardEntry): string {
        return JSON.stringify(uncast(value, r("LeaderboardEntry")), null, 2);
    }

    public static toLeaderboardUpdated(json: string): LeaderboardUpdated {
        return cast(JSON.parse(json), r("LeaderboardUpdated"));
    }

    public static leaderboardUpdatedToJson(value: LeaderboardUpdated): string {
        return JSON.stringify(uncast(value, r("LeaderboardUpdated")), null, 2);
    }

    public static toPlayerKicked(json: string): PlayerKicked {
        return cast(JSON.parse(json), r("PlayerKicked"));
    }

    public static playerKickedToJson(value: PlayerKicked): string {
        return JSON.stringify(uncast(value, r("PlayerKicked")), null, 2);
    }

    public static toPlayerSummary(json: string): PlayerSummary {
        return cast(JSON.parse(json), r("PlayerSummary"));
    }

    public static playerSummaryToJson(value: PlayerSummary): string {
        return JSON.stringify(uncast(value, r("PlayerSummary")), null, 2);
    }

    public static toPresencePlayerJoined(json: string): PresencePlayerJoined {
        return cast(JSON.parse(json), r("PresencePlayerJoined"));
    }

    public static presencePlayerJoinedToJson(value: PresencePlayerJoined): string {
        return JSON.stringify(uncast(value, r("PresencePlayerJoined")), null, 2);
    }

    public static toPresencePlayerLeft(json: string): PresencePlayerLeft {
        return cast(JSON.parse(json), r("PresencePlayerLeft"));
    }

    public static presencePlayerLeftToJson(value: PresencePlayerLeft): string {
        return JSON.stringify(uncast(value, r("PresencePlayerLeft")), null, 2);
    }

    public static toQuestionEnded(json: string): QuestionEnded {
        return cast(JSON.parse(json), r("QuestionEnded"));
    }

    public static questionEndedToJson(value: QuestionEnded): string {
        return JSON.stringify(uncast(value, r("QuestionEnded")), null, 2);
    }

    public static toQuestionOption(json: string): QuestionOption {
        return cast(JSON.parse(json), r("QuestionOption"));
    }

    public static questionOptionToJson(value: QuestionOption): string {
        return JSON.stringify(uncast(value, r("QuestionOption")), null, 2);
    }

    public static toQuestionReviewed(json: string): QuestionReviewed {
        return cast(JSON.parse(json), r("QuestionReviewed"));
    }

    public static questionReviewedToJson(value: QuestionReviewed): string {
        return JSON.stringify(uncast(value, r("QuestionReviewed")), null, 2);
    }

    public static toQuestionStarted(json: string): QuestionStarted {
        return cast(JSON.parse(json), r("QuestionStarted"));
    }

    public static questionStartedToJson(value: QuestionStarted): string {
        return JSON.stringify(uncast(value, r("QuestionStarted")), null, 2);
    }
}

function invalidValue(typ: any, val: any, key: any, parent: any = ''): never {
    const prettyTyp = prettyTypeName(typ);
    const parentText = parent ? ` on ${parent}` : '';
    const keyText = key ? ` for key "${key}"` : '';
    throw Error(`Invalid value${keyText}${parentText}. Expected ${prettyTyp} but got ${JSON.stringify(val)}`);
}

function prettyTypeName(typ: any): string {
    if (Array.isArray(typ)) {
        if (typ.length === 2 && typ[0] === undefined) {
            return `an optional ${prettyTypeName(typ[1])}`;
        } else {
            return `one of [${typ.map(a => { return prettyTypeName(a); }).join(", ")}]`;
        }
    } else if (typeof typ === "object" && typ.literal !== undefined) {
        return typ.literal;
    } else {
        return typeof typ;
    }
}

function jsonToJSProps(typ: any): any {
    if (typ.jsonToJS === undefined) {
        const map: any = {};
        typ.props.forEach((p: any) => map[p.json] = { key: p.js, typ: p.typ });
        typ.jsonToJS = map;
    }
    return typ.jsonToJS;
}

function jsToJSONProps(typ: any): any {
    if (typ.jsToJSON === undefined) {
        const map: any = {};
        typ.props.forEach((p: any) => map[p.js] = { key: p.json, typ: p.typ });
        typ.jsToJSON = map;
    }
    return typ.jsToJSON;
}

function transform(val: any, typ: any, getProps: any, key: any = '', parent: any = ''): any {
    function transformPrimitive(typ: string, val: any): any {
        if (typeof typ === typeof val) return val;
        return invalidValue(typ, val, key, parent);
    }

    function transformUnion(typs: any[], val: any): any {
        // val must validate against one typ in typs
        const l = typs.length;
        for (let i = 0; i < l; i++) {
            const typ = typs[i];
            try {
                return transform(val, typ, getProps);
            } catch (_) {}
        }
        return invalidValue(typs, val, key, parent);
    }

    function transformEnum(cases: string[], val: any): any {
        if (cases.indexOf(val) !== -1) return val;
        return invalidValue(cases.map(a => { return l(a); }), val, key, parent);
    }

    function transformArray(typ: any, val: any): any {
        // val must be an array with no invalid elements
        if (!Array.isArray(val)) return invalidValue(l("array"), val, key, parent);
        return val.map(el => transform(el, typ, getProps));
    }

    function transformDate(val: any): any {
        if (val === null) {
            return null;
        }
        const d = new Date(val);
        if (isNaN(d.valueOf())) {
            return invalidValue(l("Date"), val, key, parent);
        }
        return d;
    }

    function transformObject(props: { [k: string]: any }, additional: any, val: any): any {
        if (val === null || typeof val !== "object" || Array.isArray(val)) {
            return invalidValue(l(ref || "object"), val, key, parent);
        }
        const result: any = {};
        Object.getOwnPropertyNames(props).forEach(key => {
            const prop = props[key];
            const v = Object.prototype.hasOwnProperty.call(val, key) ? val[key] : undefined;
            result[prop.key] = transform(v, prop.typ, getProps, key, ref);
        });
        Object.getOwnPropertyNames(val).forEach(key => {
            if (!Object.prototype.hasOwnProperty.call(props, key)) {
                result[key] = transform(val[key], additional, getProps, key, ref);
            }
        });
        return result;
    }

    if (typ === "any") return val;
    if (typ === null) {
        if (val === null) return val;
        return invalidValue(typ, val, key, parent);
    }
    if (typ === false) return invalidValue(typ, val, key, parent);
    let ref: any = undefined;
    while (typeof typ === "object" && typ.ref !== undefined) {
        ref = typ.ref;
        typ = typeMap[typ.ref];
    }
    if (Array.isArray(typ)) return transformEnum(typ, val);
    if (typeof typ === "object") {
        return typ.hasOwnProperty("unionMembers") ? transformUnion(typ.unionMembers, val)
            : typ.hasOwnProperty("arrayItems")    ? transformArray(typ.arrayItems, val)
            : typ.hasOwnProperty("props")         ? transformObject(getProps(typ), typ.additional, val)
            : invalidValue(typ, val, key, parent);
    }
    // Numbers can be parsed by Date but shouldn't be.
    if (typ === Date && typeof val !== "number") return transformDate(val);
    return transformPrimitive(typ, val);
}

function cast<T>(val: any, typ: any): T {
    return transform(val, typ, jsonToJSProps);
}

function uncast<T>(val: T, typ: any): any {
    return transform(val, typ, jsToJSONProps);
}

function l(typ: any) {
    return { literal: typ };
}

function a(typ: any) {
    return { arrayItems: typ };
}

function u(...typs: any[]) {
    return { unionMembers: typs };
}

function o(props: any[], additional: any) {
    return { props, additional };
}

function m(additional: any) {
    return { props: [], additional };
}

function r(name: string) {
    return { ref: name };
}

const typeMap: any = {
    "AnswerResult": o([
        { json: "correct", js: "correct", typ: true },
        { json: "pointsAwarded", js: "pointsAwarded", typ: 0 },
        { json: "questionId", js: "questionId", typ: "" },
        { json: "totalScore", js: "totalScore", typ: 0 },
    ], "any"),
    "AnswerSubmit": o([
        { json: "optionId", js: "optionId", typ: "" },
        { json: "questionId", js: "questionId", typ: "" },
    ], "any"),
    "ErrorPayload": o([
        { json: "code", js: "code", typ: "" },
        { json: "message", js: "message", typ: "" },
    ], "any"),
    "GameEnded": o([
        { json: "endedAt", js: "endedAt", typ: Date },
        { json: "finalLeaderboard", js: "finalLeaderboard", typ: a(r("LeaderboardEntry")) },
    ], "any"),
    "LeaderboardEntry": o([
        { json: "clientId", js: "clientId", typ: "" },
        { json: "nickname", js: "nickname", typ: "" },
        { json: "rank", js: "rank", typ: 0 },
        { json: "score", js: "score", typ: 0 },
    ], "any"),
    "GameStarted": o([
        { json: "startedAt", js: "startedAt", typ: Date },
    ], "any"),
    "LeaderboardUpdated": o([
        { json: "entries", js: "entries", typ: a(r("LeaderboardEntry")) },
        { json: "questionIndex", js: "questionIndex", typ: 0 },
    ], "any"),
    "PlayerKicked": o([
        { json: "reason", js: "reason", typ: "" },
    ], "any"),
    "PresencePlayerJoined": o([
        { json: "player", js: "player", typ: r("PlayerSummary") },
        { json: "playerCount", js: "playerCount", typ: 0 },
    ], "any"),
    "PlayerSummary": o([
        { json: "clientId", js: "clientId", typ: "" },
        { json: "nickname", js: "nickname", typ: "" },
    ], "any"),
    "PresencePlayerLeft": o([
        { json: "clientId", js: "clientId", typ: "" },
        { json: "playerCount", js: "playerCount", typ: 0 },
    ], "any"),
    "QuestionEnded": o([
        { json: "answerCounts", js: "answerCounts", typ: a(r("AnswerCount")) },
        { json: "correctOptionId", js: "correctOptionId", typ: "" },
        { json: "questionId", js: "questionId", typ: "" },
        { json: "questionIndex", js: "questionIndex", typ: 0 },
    ], "any"),
    "AnswerCount": o([
        { json: "count", js: "count", typ: 0 },
        { json: "optionId", js: "optionId", typ: "" },
    ], "any"),
    "QuestionReviewed": o([
        { json: "answerCounts", js: "answerCounts", typ: a(r("AnswerCount")) },
        { json: "correctOptionId", js: "correctOptionId", typ: "" },
        { json: "options", js: "options", typ: a(r("QuestionOption")) },
        { json: "prompt", js: "prompt", typ: "" },
        { json: "questionId", js: "questionId", typ: "" },
        { json: "questionIndex", js: "questionIndex", typ: 0 },
        { json: "totalQuestions", js: "totalQuestions", typ: 0 },
    ], "any"),
    "QuestionOption": o([
        { json: "id", js: "id", typ: "" },
        { json: "text", js: "text", typ: "" },
    ], "any"),
    "QuestionStarted": o([
        { json: "options", js: "options", typ: a(r("QuestionOption")) },
        { json: "prompt", js: "prompt", typ: "" },
        { json: "questionId", js: "questionId", typ: "" },
        { json: "questionIndex", js: "questionIndex", typ: 0 },
        { json: "timed", js: "timed", typ: true },
        { json: "timeLimitSeconds", js: "timeLimitSeconds", typ: 0 },
        { json: "totalQuestions", js: "totalQuestions", typ: 0 },
    ], "any"),
};
