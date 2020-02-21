#!/usr/bin/env bash

shopt -s extglob
set -o errtrace
set -o errexit
set +o noclobber

# Defaults {{{
ASSUMEYES=0
NOOP=
FORCE=0
VERBOSE=1
DEST=tmp

GENDOC=0
BUILD=1
DEPLOY=
DOCKER_BUILD=
DOCKER_PUBLISH=
K8S_DEPLOY=
MICROK8S_DEPLOY=
TEST=0
# }}}

#General variables {{{
ARGS=()
APPNAME=$(awk '/^[ \t]*APP += +/{print $3}' version.go | tr -d '"')
VERSION=$(awk '/VERSION +=/{print $4}' version.go | tr -d '"')
BUILDINFO=""
DOCKER_REPO=${DOCKER_REPO:-docker.apac.inin.com}
DOCKER_APP=${DOCKER_APP:-sim}
DOCKER_SERVICE=${APPNAME}
HELM_RELEASE=${HELM_RELEASE:-dev}
# }}}

# Tools {{{
DOCKER=docker
KUBECTL=kubectl
BAR="bar --no-summary --no-time --no-throughput --no-count"
JQ=jq
MD5=md5sum
SHA=shasum
STAT="stat --format %s"
TAR=tar
UUID=uuidgen
TOUPPER="tr '[:lower:]' '[:upper:]'"

[[ $OSNAME == 'darwin' ]] && MD5="md5"
[[ $OSNAME == 'darwin' ]] && BAR="bar -n"
[[ $OSNAME == 'darwin' ]] && STAT="stat -n -f %z"
[[ $OSNAME == 'darwin' ]] && TAR="gtar"
[[ $OSNAME == 'darwin' ]] && UUID="uuidgen"
# }}}

function trace() { # {{{2
  [[ $VERBOSE > 1 ]] && echo -e "$@"
} # 2}}}

function verbose() { # {{{2
  [[ $VERBOSE > 0 ]] && echo -e "$@"
} # 2}}}

function warn() { # {{{2
  echo -e "Warning: $@"
} # 2}}}

function error() { # {{{2
  echo -e "\e[0;31mError: $@\e[0m" >&2
} # 2}}}

function die() { # {{{2
  local message=$1
  local errorlevel=$2

  [[ -z $message    ]] && message='Died'
  [[ -z $errorlevel ]] && errorlevel=1
  echo -e "\e[0;31m$message\e[0m" >&2
  exit $errorlevel
} # 2}}}

function die_on_error() { # {{{2
  local status=$?
  local message=$1

  if [[ $status != 0 ]]; then
    die "${message}, Error: $status" $status
  fi
} # 2}}}

function build() { # {{{2
  local output

  for GOOS in darwin linux windows; do
    [[ -d bin/$GOOS ]] || mkdir -p bin/$GOOS
    for GOARCH in amd64; do # we do not care about 386 anymore
      verbose "Building ${APPNAME} for $GOOS-$GOARCH"
      output="bin/$GOOS/$APPNAME"
      [[ $GOOS == "windows" ]] && output="${output}.exe"
      if [[ -n $BUILDINFO ]]; then
        CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-X main.commit=${BUILDINFO}" -o $output .
      else
        CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -o $output .
      fi
      file $output
    done
  done

  # Special command for Raspberry Pi
    [[ -d bin/pi ]] || mkdir -p bin/pi
    verbose "Building ${APPNAME} for Raspberry Pi"
    output="bin/pi/$APPNAME"
    if [[ -n $BUILDINFO ]]; then
      CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -ldflags "-X main.commit=${BUILDINFO}" -o $output .
    else
      CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -o $output .
    fi
    file $output

  return 0
} # 2}}}

function docker_build() { # {{{2
  local docker_image=${DOCKER_SERVICE}
  local docker_tag=${VERSION/\+/-}

  verbose "Building Docker image for application $APPNAME (Docker tag: $docker_tag)"
  $NOOP $DOCKER build -t ${docker_image}:${docker_tag} .
  status=$? && [[ $status != 0 ]] && die "Failed building service $service, error: $status" $status
  if [[ $BUILD == "prod" ]]; then
    $NOOP $DOCKER tag ${docker_image}:${docker_tag} ${docker_image}:latest
    status=$? && [[ $status != 0 ]] && die "Failed tagging service $service to 'latest', error: $status" $status
  elif [[ $BUILD == "dev" ]]; then
    $NOOP $DOCKER tag ${docker_image}:${docker_tag} ${docker_image}:dev
    status=$? && [[ $status != 0 ]] && die "Failed tagging service $service to 'latest', error: $status" $status
  fi

  $DOCKER images | grep "${DOCKER_SERVICE}"

  return 0
} # 2}}}

function docker_publish() { # {{{2
  local docker_image=${DOCKER_REPO}/${DOCKER_APP}/${DOCKER_SERVICE}
  local docker_tag=${VERSION/\+/-}

  verbose "Pushing Docker images to ${DOCKER_REPO}"
  if [[ $BUILD == "prod" ]]; then
    $NOOP $DOCKER tag ${DOCKER_SERVICE}:${docker_tag} ${docker_image}:${docker_tag}
    status=$? && [[ $status != 0 ]] && die "Failed tagging service $service to 'latest', error: $status" $status
    $NOOP $DOCKER tag ${DOCKER_SERVICE}:${docker_tag} ${docker_image}:latest
    status=$? && [[ $status != 0 ]] && die "Failed tagging service $service to 'latest', error: $status" $status
    verbose "Pushing Docker image $docker_image to the registry"
    $NOOP $DOCKER push ${docker_image}:${docker_tag}
    status=$? && [[ $status != 0 ]] && die "Failed pushing service $service image to the registry, error: $status" $status
    $NOOP $DOCKER push ${docker_image}:latest
    status=$? && [[ $status != 0 ]] && die "Failed pushing service $service image to the registry, error: $status" $status
  elif [[ $BUILD == "dev" ]]; then
    $NOOP $DOCKER tag ${DOCKER_SERVICE}:${docker_tag} ${docker_image}:dev
    status=$? && [[ $status != 0 ]] && die "Failed tagging service $service to 'latest', error: $status" $status
    verbose "Pushing Docker image ${docker_image}:dev to the registry"
    $NOOP $DOCKER push ${docker_image}:dev
    status=$? && [[ $status != 0 ]] && die "Failed pushing service $service image to the registry, error: $status" $status
  fi

  return 0
} # 2}}}

function microk8s_deploy() { # {{{2
  local docker_image=${DOCKER_SERVICE}
  local namespace=${DOCKER_APP}
  local tier=${DOCKER_SERVICE/_/-}

  verbose "Importing Docker image to microk8s"
  $NOOP $DOCKER save $docker_image | microk8s.ctr -n k8s.io image import -

  if [[ -n $($KUBECTL get pods --namespace ${namespace} --selector app.kubernetes.io/name=${tier} 2> /dev/null) ]]; then
    verbose "Cycling pods for tier ${namspace}/${tier}"
    $KUBECTL delete pod --namespace ${namespace} --selector app.kubernetes.io/name=${tier}
    $KUBECTL rollout status $($KUBECTL get deployments.apps --namespace ${namespace} --selector app.kubernetes.io/name=${tier} -o name) --namespace ${namespace}
    status=$? && [[ $status != 0 ]] && die "Failed importing service $service image to microk8s, error: $status" $status
  fi
} # 2}}}

function k8s_deploy() { # {{{2
  local namespace=${DOCKER_APP}
  local name=${DOCKER_SERVICE/_/-}
  verbose "Cycling pods for ${namespace}/${name}"
  $KUBECTL delete pod --namespace ${namespace} --selector app.kubernetes.io/name=${name},app.kubernetes.io/instance=${HELM_RELEASE}
  $KUBECTL rollout status deployment --namespace ${namespace} ${HELM_RELEASE}-${name}
  status=$? && [[ $status != 0 ]] && die "Failed importing service $service image to Kubernetes, error: $status" $status
  return 0
} # 2}}}

function test_ci() { # {{{2
  shift
  nodemon --ext go,json --delay 5  --ignore .git --ignore bin --exec go test -run "$@"
} # 2}}}

function usage() { # {{{2
  echo "$(basename $0) [options]"
  echo "  Builds, packages and deploys the app"
  echo "  Options are:"
  echo " --deploy=scp_url"
  echo "   Deploy the service to a remote VM via scp and ssh"
  echo "   the ssh URL must be: user@host:path"
  echo " --docker-build"
  echo "   Build the Docker images"
  echo " --docker-publish"
  echo "   Build the Docker images and Deploy them to the Genesys Registry"
  echo " --k8s-deploy"
  echo "   Build/Deploy the Docker images and recycle the corresponding pods on the current Kubernetes cluster"
  echo " --microk8s-deploy"
  echo "   Build/Deploy the Docker images to microk8s and recycle the corresponding pods on the current Kubernetes cluster"
  echo " --no-build"
  echo "   Do not build the app nor the docker images."
  echo " --help, -h, -?  "
  echo "   Prints some help on the output."
  echo " --noop, --dry-run  "
  echo "   Do not execute instructions that would make changes to the system (write files, install software, etc)."
  echo " --quiet  "
  echo "   Runs the script as silently as possible."
  echo " --verbose  "
  echo "   Runs the script verbosely, that's by default."
  echo " --yes, --assumeyes, -y  "
  echo "   Answers yes to any questions automatiquely."
} # 2}}}

function parse_args() { # {{{2
  local branch

  while (( "$#" )); do
    # Replace --parm=arg with --parm arg
    [[ $1 == --*=* ]] && set -- "${1%%=*}" "${1#*=}" "${@:2}"
    case $1 in
      --deploy)
        [[ -z $2 || ${2:0:1} == '-' ]] && die "Argument for option $1 is missing"
        DEPLOY=$2
        shift 2
        continue
        ;;
      --docker_build|--docker-build)
        DOCKER_BUILD=1
        GENDOC=0
        ;;
      --docker_publish|--docker-publish)
        DOCKER_BUILD=1
        DOCKER_PUBLISH=1
        GENDOC=0
        [[ -n $2 ]] && DOCKER_REPO=$2 && shift 2 && continue
        ;;
      --helm_publish|--helm-publish)
        HELM_PUBLISH=1
        BUILD=none
        GENDOC=0
        ;;
      --k8s_up|--k8s-up|--k8s_push|--k8s-push|--k8s_deploy|--k8s-deploy)
        DOCKER_BUILD=1
        DOCKER_PUBLISH=1
        K8S_DEPLOY=1
        GENDOC=0
        [[ -n $2 ]] && DOCKER_REPO=$2 && shift 2 && continue
        ;;
      --microk8s_deploy|--microk8s-deploy)
        DOCKER_BUILD=1
        MICROK8S_DEPLOY=1
        GENDOC=0
        ;;
      --no-build|--no_build)
        BUILD=none
        DOCKER_BUILD=0
        ;;
      --gendoc)
        GENDOC=1
        ;;
      --no-gendoc|--no_gendoc)
        GENDOC=0
        ;;
      --test|--test_ci|--test-ci)
        TEST=1
        BUILD=none
        ;;

      # Standard options
      --force)
        warn "This program will overwrite the current configuration"
        FORCE=1
        ;;
      -h|-\?|--help)
       trace "Showing usage"
       usage
       exit 0
       ;;
      --noop|--dry_run|--dry-run)
        warn "This program will execute in dry mode, your system will not be modified"
        NOOP=:
        ;;
     --quiet)
       VERBOSE=0
       trace "Verbose level: $VERBOSE"
       ;;
     -v|--verbose)
       VERBOSE=$((VERBOSE + 1))
       trace "Verbose level: $VERBOSE"
       ;;
     -y|--yes|--assumeyes|--assume_yes|--assume-yes) # All questions will get a "yes"  answer automatically
       ASSUMEYES=1
       trace "All prompts will be answered \"yes\" automatically"
       ;;
     -?*) # Invalid options
       warn "Unknown option $1 will be ignored"
       ;;
     --) # Force end of options
       shift
       break
       ;;
     *)  # End of options
       ARGS+=( "$1" )
       break
       ;;
    esac
    shift
  done

  # Set all positional arguments back in the proper order
  eval set -- "${ARGS[@]}"

  if [[ -n $1 ]]; then
    if [[ $1 =~ ".*/.*" ]]; then
      DEST="${1%/*}"
      FILENAME="${1##*/}"
    else
      FILENAME=$1
    fi
  fi
  if [[ $BUILD == "1" ]]; then
    branch=$(git symbolic-ref --short HEAD)
    if [[ $branch == "master" ]]; then
      BUILD="production"
    else
      BUILD="development"
    fi
  fi
  return 0
} # 2}}}

function main() { # {{{2
  parse_args "$@"

  mkdir -p $DEST

  case $BUILD in
    production|prod|1)
      BUILD=prod
      ;;
    dev|development)
      BUILD=dev
      BUILDINFO="+$(date +'%Y%m%d').$(git rev-parse --short HEAD)"
      VERSION="${VERSION}${BUILDINFO}"
      FILENAME="${FILEBASE}-${VERSION}"
      ;;
    none)
      ;;
    *)
      die "Invalid Build environment: $BUILD"
      ;;
  esac

  verbose Packaging version $VERSION

  if [[ $GENDOC == 1 ]]; then
    if command -v pandoc 2>&1 > /dev/null ; then
      verbose "Generating PDF documentation"
      $NOOP pandoc --standalone --pdf-engine=xelatex --toc --top-level-division=chapter -o $DEST/${FILEBASE}.pdf README.yaml README.md
      die_on_error "Failed to generate the documentation"
      [[ -f $DEST/${FILEBASE}.pdf ]] && cp $DEST/${FILEBASE}.pdf ~/Dropbox/Genesys/LINE
    fi
  fi

# Don't forget to run the following the first time:
# go get -u ./...

  [[ $BUILD != 'none' ]]      && (build           ; die_on_error "Failed while building")
  [[ $DOCKER_BUILD == 1 ]]    && (docker_build    ; die_on_error "Failed while building for Docker")
  [[ $DOCKER_PUBLISH == 1 ]]  && (docker_publish  ; die_on_error "Failed while publishing to Docker")
  [[ $MICROK8S_DEPLOY == 1 ]] && (microk8s_deploy ; die_on_error "Failed while deplyoing to MicroK8s")
  [[ $K8S_DEPLOY == 1 ]]      && (k8s_deploy      ; die_on_error "Failed while deplyoing to Kubernetes")
  [[ $TEST == 1 ]] && test_ci "$@"
  exit 0
} # 2}}}

main "$@"
