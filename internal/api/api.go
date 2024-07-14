package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"journey/internal/api/spec"
	"journey/internal/pgstore"
	"net/http"

	"github.com/discord-gophers/goapi-gen/types"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type store interface {
	CreateActivity(ctx context.Context, arg pgstore.CreateActivityParams) (uuid.UUID, error)
	CreateTripLink(ctx context.Context, arg pgstore.CreateTripLinkParams) (uuid.UUID, error)
	CreateTrip(ctx context.Context, pool *pgxpool.Pool, params spec.CreateTripRequest) (uuid.UUID, error)

	ConfirmParticipant(ctx context.Context, participantID uuid.UUID) error

	GetParticipant(ctx context.Context, particpantID uuid.UUID) (pgstore.Participant, error)
	GetParticipants(ctx context.Context, tripID uuid.UUID) ([]pgstore.Participant, error)
	GetTrip(ctx context.Context, tripID uuid.UUID) (pgstore.Trip, error)
	GetTripActivities(ctx context.Context, tripID uuid.UUID) ([]pgstore.Activity, error)
	GetTripLinks(ctx context.Context, tripID uuid.UUID) ([]pgstore.Link, error)

	InviteParticipantsToTrip(ctx context.Context, arg []pgstore.InviteParticipantsToTripParams) (int64, error)

	UpdateTrip(ctx context.Context, params pgstore.UpdateTripParams) error
}

type mailer interface {
	SendConfirmTripEmailToTripOwner(tripID uuid.UUID) error
}

type API struct {
	store     store
	logger    *zap.Logger
	validator *validator.Validate
	pool      *pgxpool.Pool
	mailer    mailer
}

func NewAPI(pool *pgxpool.Pool, logger *zap.Logger, mailer mailer) API {
	validator := validator.New(validator.WithRequiredStructEnabled())

	return API{pgstore.New(pool), logger, validator, pool, mailer}
}

// Confirms a participant on a trip.
// (PATCH /participants/{participantId}/confirm)
func (api API) PatchParticipantsParticipantIDConfirm(
	w http.ResponseWriter,
	r *http.Request,
	participantID string,
) *spec.Response {
	id, err := uuid.Parse(participantID)
	if err != nil {
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(
			spec.Error{Message: "uuid invalid"},
		)
	}

	participant, err := api.store.GetParticipant(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PatchParticipantsParticipantIDConfirmJSON400Response(
				spec.Error{Message: "participant not found"},
			)
		}

		api.logger.Error("failed to get participant", zap.Error(err), zap.String("participant_id", participantID))
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	if participant.IsConfirmed {
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(
			spec.Error{Message: "participant ja confirmado"},
		)
	}

	if err := api.store.ConfirmParticipant(r.Context(), id); err != nil {
		api.logger.Error("failed to confirm participant", zap.Error(err), zap.String("participant_id", participantID))
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	return spec.PatchParticipantsParticipantIDConfirmJSON204Response(nil)
}

// Create a new trip
// (POST /trips)
func (api API) PostTrips(w http.ResponseWriter, r *http.Request) *spec.Response {
	var body spec.CreateTripRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsJSON400Response(spec.Error{Message: "invalid JSON: " + err.Error()})
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.PostTripsJSON400Response(spec.Error{Message: "invalid input: " + err.Error()})
	}

	tripID, err := api.store.CreateTrip(r.Context(), api.pool, body)
	if err != nil {
		api.logger.Error("failed to create trip", zap.Error(err), zap.String("trip: ", fmt.Sprintf("%s", body)))
		return spec.PostTripsJSON400Response(spec.Error{Message: "failed to create trip, try again"})
	}

	go func() {
		if err := api.mailer.SendConfirmTripEmailToTripOwner(tripID); err != nil {
			api.logger.Error(
				"failed to send email on PostTrips",
				zap.Error(err),
				zap.String("trip_id", tripID.String()),
			)
		}
	}()

	return spec.PostTripsJSON201Response(spec.CreateTripResponse{TripID: tripID.String()})
}

// Get a trip details.
// (GET /trips/{tripId})
func (api API) GetTripsTripID(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDJSON400Response(
			spec.Error{Message: "uuid invalid"},
		)
	}

	trip, err := api.store.GetTrip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDJSON400Response(
				spec.Error{Message: "trip not found"},
			)
		}

		api.logger.Error("failed to get trip", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	return spec.GetTripsTripIDJSON200Response(
		spec.GetTripDetailsResponse{
			Trip: spec.GetTripDetailsResponseTripObj{
				Destination: trip.Destination,
				EndsAt:      trip.EndsAt.Time,
				ID:          trip.ID.String(),
				IsConfirmed: trip.IsConfirmed,
				StartsAt:    trip.StartsAt.Time,
			},
		},
	)
}

// Update a trip.
// (PUT /trips/{tripId})
func (api API) PutTripsTripID(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PutTripsTripIDJSON400Response(
			spec.Error{Message: "uuid invalid"},
		)
	}

	trip, err := api.store.GetTrip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PutTripsTripIDJSON400Response(
				spec.Error{Message: "trip not found"},
			)
		}

		api.logger.Error("failed to get a trip", zap.Error(err), zap.String("trip_id", tripID))
		return spec.PutTripsTripIDJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	var body pgstore.UpdateTripParams
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PutTripsTripIDJSON400Response(
			spec.Error{Message: "invalid JSON: " + err.Error()},
		)
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.PutTripsTripIDJSON400Response(
			spec.Error{Message: "invalid input: " + err.Error()},
		)
	}

	body.ID = id
	if err := api.store.UpdateTrip(r.Context(), body); err != nil {
		api.logger.Error("failed to update trip", zap.Error(err), zap.String("trip: ", fmt.Sprint(body)))
		return spec.PutTripsTripIDJSON400Response(
			spec.Error{Message: "failed to update trip, try again"},
		)
	}

	return spec.PutTripsTripIDJSON204Response(
		spec.PutTripsTripIDJSON204Response(trip),
	)
}

// Get a trip activities.
// (GET /trips/{tripId}/activities)
func (api API) GetTripsTripIDActivities(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDActivitiesJSON400Response(
			spec.Error{Message: "uuid invalid"},
		)
	}

	trip, err := api.store.GetTrip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDActivitiesJSON400Response(
				spec.Error{Message: "trip not found"},
			)
		}

		api.logger.Error("failed do get trip", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDActivitiesJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	activities, err := api.store.GetTripActivities(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDActivitiesJSON400Response(
				spec.Error{Message: "trip activities not found"},
			)
		}

		api.logger.Error("failed do get trip activities", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDActivitiesJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	var responseActivities = []spec.GetTripActivitiesResponseInnerArray{}
	for _, activity := range activities {
		responseActivities = append(
			responseActivities,
			spec.GetTripActivitiesResponseInnerArray{
				ID:       activity.ID.String(),
				OccursAt: activity.OccursAt.Time,
				Title:    activity.Title,
			},
		)
	}

	return spec.GetTripsTripIDActivitiesJSON200Response(
		spec.GetTripActivitiesResponse{
			Activities: []spec.GetTripActivitiesResponseOuterArray{
				{
					Activities: responseActivities,
					Date:       trip.StartsAt.Time,
				},
			},
		},
	)
}

// Create a trip activity.
// (POST /trips/{tripId}/activities)
func (api API) PostTripsTripIDActivities(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsTripIDActivitiesJSON400Response(
			spec.Error{Message: "uuid invalid"},
		)
	}

	if _, err := api.store.GetTrip(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PostTripsTripIDActivitiesJSON400Response(
				spec.Error{Message: "trip not found"},
			)
		}

		api.logger.Error("failed do get trip", zap.Error(err), zap.String("trip_id", tripID))
		return spec.PostTripsTripIDActivitiesJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	var body = pgstore.CreateActivityParams{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsTripIDActivitiesJSON400Response(
			spec.Error{Message: "invalid json " + err.Error()},
		)
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.PostTripsTripIDActivitiesJSON400Response(
			spec.Error{Message: "invalid input " + err.Error()},
		)
	}

	body.TripID = id
	activityId, err := api.store.CreateActivity(r.Context(), body)
	if err != nil {
		api.logger.Error("failed to create an activity", zap.Error(err), zap.String("activity: ", fmt.Sprint(body)))
		return spec.PostTripsTripIDActivitiesJSON400Response(
			spec.Error{Message: "failed to create an activity, try again"},
		)
	}

	return spec.PostTripsTripIDActivitiesJSON201Response(
		spec.CreateActivityResponse{
			ActivityID: activityId.String(),
		},
	)
}

// Confirm a trip and send e-mail invitations.
// (GET /trips/{tripId}/confirm)
func (api API) GetTripsTripIDConfirm(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	panic("not implemented") // TODO: Implement
}

// Invite someone to the trip.
// (POST /trips/{tripId}/invites)
func (api API) PostTripsTripIDInvites(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsTripIDInvitesJSON400Response(
			spec.Error{Message: "uuid invalid"},
		)
	}

	if _, err := api.store.GetTrip(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PostTripsTripIDInvitesJSON400Response(
				spec.Error{Message: "trip not found"},
			)
		}

		api.logger.Error("failed do get trip", zap.Error(err), zap.String("trip_id", tripID))
		return spec.PostTripsTripIDInvitesJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	var body = pgstore.InviteParticipantsToTripParams{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsTripIDInvitesJSON400Response(
			spec.Error{Message: "invalid json " + err.Error()},
		)
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.PostTripsTripIDInvitesJSON400Response(
			spec.Error{Message: "invalid input " + err.Error()},
		)
	}

	body.TripID = id
	if _, err = api.store.InviteParticipantsToTrip(
		r.Context(),
		[]pgstore.InviteParticipantsToTripParams{
			body,
		},
	); err != nil {
		api.logger.Error("failed to invite participant", zap.Error(err), zap.String("activity: ", fmt.Sprint(body)))
		return spec.PostTripsTripIDInvitesJSON400Response(
			spec.Error{Message: "failed to invite participant, try again"},
		)
	}

	return spec.PostTripsTripIDInvitesJSON201Response(nil)
}

// Get a trip links.
// (GET /trips/{tripId}/links)
func (api API) GetTripsTripIDLinks(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDLinksJSON400Response(
			spec.Error{Message: "uuid invalid"},
		)
	}

	if _, err := api.store.GetTrip(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDLinksJSON400Response(
				spec.Error{Message: "trip not found"},
			)
		}

		api.logger.Error("failed do get trip", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDLinksJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	links, err := api.store.GetTripLinks(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDLinksJSON400Response(
				spec.Error{Message: "link not found"},
			)
		}
	}

	var responseLink = []spec.GetLinksResponseArray{}
	for _, v := range links {
		responseLink = append(
			responseLink,
			spec.GetLinksResponseArray{
				ID:    v.ID.String(),
				Title: v.Title,
				URL:   v.Url,
			},
		)
	}

	return spec.GetTripsTripIDLinksJSON200Response(
		spec.GetLinksResponse{
			Links: responseLink,
		},
	)
}

// Create a trip link.
// (POST /trips/{tripId}/links)
func (api API) PostTripsTripIDLinks(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsTripIDLinksJSON400Response(
			spec.Error{Message: "uuid invalid"},
		)
	}

	if _, err := api.store.GetTrip(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PostTripsTripIDLinksJSON400Response(
				spec.Error{Message: "trip not found"},
			)
		}

		api.logger.Error("failed do get trip", zap.Error(err), zap.String("trip_id", tripID))
		return spec.PostTripsTripIDLinksJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	var body = pgstore.CreateTripLinkParams{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsTripIDLinksJSON400Response(
			spec.Error{Message: "invalid json " + err.Error()},
		)
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.PostTripsTripIDLinksJSON400Response(
			spec.Error{Message: "invalid input " + err.Error()},
		)
	}

	body.TripID = id
	linkId, err := api.store.CreateTripLink(r.Context(), body)
	if err != nil {
		api.logger.Error("failed to create a link", zap.Error(err), zap.String("link: ", fmt.Sprint(body)))
		return spec.PostTripsTripIDLinksJSON400Response(
			spec.Error{Message: "failed to create a link, try again"},
		)
	}

	return spec.PostTripsTripIDLinksJSON201Response(
		spec.CreateLinkResponse{
			LinkID: linkId.String(),
		},
	)
}

// Get a trip participants.
// (GET /trips/{tripId}/participants)
func (api API) GetTripsTripIDParticipants(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDParticipantsJSON400Response(
			spec.Error{Message: "uuid invalid"},
		)
	}

	participants, err := api.store.GetParticipants(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDParticipantsJSON400Response(
				spec.Error{Message: "participants not found"},
			)
		}

		api.logger.Error("failed to get participants", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	var responsePartipants = []spec.GetTripParticipantsResponseArray{}

	for _, v := range participants {
		responsePartipants = append(
			responsePartipants,
			spec.GetTripParticipantsResponseArray{
				Email:       types.Email(v.Email),
				ID:          v.ID.String(),
				IsConfirmed: v.IsConfirmed,
				Name:        nil,
			},
		)
	}

	return spec.GetTripsTripIDParticipantsJSON200Response(
		spec.GetTripParticipantsResponse{
			Participants: responsePartipants,
		},
	)
}
