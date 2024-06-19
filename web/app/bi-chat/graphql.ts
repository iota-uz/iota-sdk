import {gql} from "@apollo/client";

const GET_DIALOGUES = gql`
    query GetDialogues {
        dialogues(limit: 100, offset: 0) {
            data {
                id
                label
            }
        }
    }
`;

const GET_DIALOGUE = gql`
    query GetDialogue($id: ID!) {
        dialogue(id: $id) {
            id
            messages {
                role
                content
            }
        }
    }
`;

const DELETE_DIALOGUE = gql`
    mutation DeleteDialogue($id: ID!) {
        deleteDialogue(id: $id)
    }
`;

const NEW_DIALOGUE = gql`
    mutation NewDialogue($input: NewDialogue!) {
        newDialogue(input: $input) {
            id
        }
    }
`;

const REPLY_TO_DIALOGUE = gql`
    mutation ReplyToDialogue($id: ID!, $input: DialogueReply!) {
        replyDialogue(id: $id, input: $input) {
            id
        }
    }
`;

const ON_DIALOGUE_CREATED = gql`
    subscription SubscribeToDialogues {
        dialogueCreated {
            id
            label
            messages {
                role
                content
            }
        }
    }
`;

const ON_DIALOGUE_UPDATED = gql`
    subscription SubscribeToDialogues {
        dialogueUpdated {
            id
            messages {
                role
                content
            }
        }
    }
`;

export {
    GET_DIALOGUES,
    GET_DIALOGUE,
    DELETE_DIALOGUE,
    NEW_DIALOGUE,
    REPLY_TO_DIALOGUE,
    ON_DIALOGUE_CREATED,
    ON_DIALOGUE_UPDATED
}